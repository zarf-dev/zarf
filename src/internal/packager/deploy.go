package packager

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/defenseunicorns/zarf/src/types"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/internal/git"
	"github.com/defenseunicorns/zarf/src/internal/helm"
	"github.com/defenseunicorns/zarf/src/internal/images"
	"github.com/defenseunicorns/zarf/src/internal/k8s"
	"github.com/defenseunicorns/zarf/src/internal/message"
	"github.com/defenseunicorns/zarf/src/internal/template"
	"github.com/defenseunicorns/zarf/src/internal/utils"
	"github.com/mholt/archiver/v3"
	"github.com/otiai10/copy"
	"github.com/pterm/pterm"
)

var valueTemplate template.Values
var connectStrings = make(types.ConnectStrings)

func Deploy() {
	message.Debug("packager.Deploy()")

	tempPath := createPaths()
	defer tempPath.clean()

	spinner := message.NewProgressSpinner("Preparing zarf package %s", config.DeployOptions.PackagePath)
	defer spinner.Stop()

	// Make sure the user gave us a package we can work with
	if utils.InvalidPath(config.DeployOptions.PackagePath) {
		spinner.Fatalf(nil, "Unable to find the package on the local system, expected package at %s", config.DeployOptions.PackagePath)
	}

	// Extract the archive
	spinner.Updatef("Extracting the package, this may take a few moments")
	err := archiver.Unarchive(config.DeployOptions.PackagePath, tempPath.base)
	if err != nil {
		spinner.Fatalf(err, "Unable to extract the package contents")
	}

	// Load the config from the extracted archive zarf.yaml
	spinner.Updatef("Loading the zarf package config")
	if err := config.LoadConfig(tempPath.base+"/zarf.yaml", true); err != nil {
		spinner.Fatalf(err, "Invalid or unreadable zarf.yaml file in %s", tempPath.base)
	}

	if config.IsZarfInitConfig() {
		// If init config, make sure things are ready
		utils.RunPreflightChecks()
	}

	spinner.Success()

	sbomViewFiles, _ := filepath.Glob(tempPath.sboms + "/sbom-viewer-*")
	// If SBOM files exist, temporary place them in the deploy directory
	if len(sbomViewFiles) > 0 {
		sbomDir := "zarf-sbom"
		// Cleanup any failed prior removals
		_ = os.RemoveAll(sbomDir)
		// Create the directory again
		utils.CreateDirectory(sbomDir, 0755)
		for _, file := range sbomViewFiles {
			// Our file copy lib explodes on these files for some reason...
			data, err := ioutil.ReadFile(file)
			if err != nil {
				message.Fatalf(err, "Unable to read the sbom-viewer file %s", file)
			}
			dst := filepath.Join(sbomDir, filepath.Base(file))
			err = ioutil.WriteFile(dst, data, 0644)
			if err != nil {
				message.Fatalf(err, "Unable to write the sbom-viewer file %s", dst)
			}
		}
	}

	// Confirm the overall package deployment
	configPath := tempPath.base + "/zarf.yaml"
	confirm := confirmAction(configPath, "Deploy", sbomViewFiles)

	// Don't continue unless the user says so
	if !confirm {
		os.Exit(0)
	}

	// Verify the components requested all exist
	components := config.GetComponents()
	var requestedComponents []string
	if config.DeployOptions.Components != "" {
		requestedComponents = strings.Split(config.DeployOptions.Components, ",")
	}
	componentsToDeploy := getValidComponents(components, requestedComponents)

	// Deploy all the components
	for _, component := range componentsToDeploy {
		deployComponents(tempPath, component)
	}

	message.PrintConnectStringTable(connectStrings)

	pterm.Success.Println("Zarf deployment complete")
	pterm.Println()

	if config.IsZarfInitConfig() {
		loginTable := pterm.TableData{
			{"     Application", "Username", "Password", "Connect"},
			{"     Registry", config.ZarfRegistryPushUser, config.GetSecret(config.StateRegistryPush), "zarf connect registry"},
		}
		for _, component := range componentsToDeploy {
			// Show message if including logging stack
			if component.Name == "logging" {
				loginTable = append(loginTable, pterm.TableData{{"     Logging", "zarf-admin", config.GetSecret(config.StateLogging), "zarf connect logging"}}...)
			}
			// Show message if including git-server
			if component.Name == "git-server" {
				loginTable = append(loginTable, pterm.TableData{
					{"     Git", config.ZarfGitPushUser, config.GetSecret(config.StateGitPush), "zarf connect git"},
					{"     Git (read-only)", config.ZarfGitReadUser, config.GetSecret(config.StateGitPull), "zarf connect git"},
				}...)
			}
		}
		_ = pterm.DefaultTable.WithHasHeader().WithData(loginTable).Render()
	}

	// All done
	os.Exit(0)
}

func deployComponents(tempPath tempPaths, component types.ZarfComponent) {
	message.Debugf("packager.deployComponents(%#v, %#v", tempPath, component)
	componentPath := createComponentPaths(tempPath.components, component)
	isSeedRegistry := config.IsZarfInitConfig() && component.Name == "zarf-seed-registry"
	hasImages := len(component.Images) > 0
	hasCharts := len(component.Charts) > 0
	hasManifests := len(component.Manifests) > 0
	hasRepos := len(component.Repos) > 0

	// All components now require a name
	message.HeaderInfof("📦 %s COMPONENT", strings.ToUpper(component.Name))

	for _, script := range component.Scripts.Before {
		loopScriptUntilSuccess(script, component.Scripts)
	}

	if len(component.Files) > 0 {
		spinner := message.NewProgressSpinner("Copying %d files", len(component.Files))
		defer spinner.Stop()

		for index, file := range component.Files {
			spinner.Updatef("Loading %s", file.Target)
			sourceFile := componentPath.files + "/" + strconv.Itoa(index)

			// If a shasum is specified check it again on deployment as well
			if file.Shasum != "" {
				spinner.Updatef("Validating SHASUM for %s", file.Target)
				utils.ValidateSha256Sum(file.Shasum, sourceFile)
			}

			// Replace temp target directories
			file.Target = strings.Replace(file.Target, "###ZARF_TEMP###", tempPath.base, 1)

			// Copy the file to the destination
			spinner.Updatef("Saving %s", file.Target)
			err := copy.Copy(sourceFile, file.Target)
			if err != nil {
				spinner.Fatalf(err, "Unable to copy the contents of %s", file.Target)
			}

			// Loop over all symlinks and create them
			for _, link := range file.Symlinks {
				spinner.Updatef("Adding symlink %s->%s", link, file.Target)
				// Try to remove the filepath if it exists
				_ = os.RemoveAll(link)
				// Make sure the parent directory exists
				_ = utils.CreateFilePath(link)
				// Create the symlink
				err := os.Symlink(file.Target, link)
				if err != nil {
					spinner.Fatalf(err, "Unable to create the symbolic link %s -> %s", link, file.Target)
				}
			}

			// Cleanup now to reduce disk pressure
			_ = os.RemoveAll(sourceFile)
		}
		spinner.Success()
	}

	// Start any data injection async
	if len(component.DataInjections) > 0 {
		var waitGroup sync.WaitGroup

		message.Info("Loading data injections")
		for _, data := range component.DataInjections {
			waitGroup.Add(1)
			go handleDataInjection(&waitGroup, data, componentPath)
		}
		defer waitGroup.Wait()
	}

	if isSeedRegistry {
		preSeedRegistry(tempPath)
		valueTemplate = template.Generate()
	}

	if !valueTemplate.Ready() && (hasImages || hasCharts || hasManifests || hasRepos) {
		// If we are touching K8s, make sure we can talk to it once per deployment
		spinner := message.NewProgressSpinner("Loading the Zarf State from the Kubernetes cluster")
		defer spinner.Stop()

		state := k8s.LoadZarfState()

		if state.Distro == "" {
			// If no distro the zarf secret did not load properly
			spinner.Fatalf(nil, "Unable to load the zarf/zarf-state secret, did you remember to run zarf init first?")
		}

		// Continue loading state data if it is valid
		config.InitState(state)
		valueTemplate = template.Generate()

		if hasImages && state.Architecture != config.GetArch() {
			// If the package has images but the architectures don't match warn the user to avoid ugly hidden errors with image push/pull
			spinner.Fatalf(nil, "This package architecture is %s, but this cluster seems to be initialized with the %s architecture",
				config.GetArch(),
				state.Architecture)
		}

		spinner.Success()
	}

	if hasImages {
		// Try image push up to 3 times
		for retry := 0; retry < 3; retry++ {
			if err := images.PushToZarfRegistry(tempPath.images, component.Images); err != nil {
				message.Errorf(err, "Unable to push images to the Zarf Registry, retrying in 5 seconds...")
				time.Sleep(5 * time.Second)
				continue
			} else {
				break
			}
		}

	}

	if hasRepos {
		// Try repo push up to 3 times
		for retry := 0; retry < 3; retry++ {
			// Push all the repos from the extracted archive
			if err := git.PushAllDirectories(componentPath.repos); err != nil {
				message.Errorf(err, "Unable to push repos to the Zarf Registry, retrying in 5 seconds...")
				time.Sleep(5 * time.Second)
				continue
			} else {
				break
			}
		}
	}

	for _, chart := range component.Charts {
		// zarf magic for the value file
		for idx := range chart.ValuesFiles {
			chartValueName := helm.StandardName(componentPath.values, chart) + "-" + strconv.Itoa(idx)
			valueTemplate.Apply(component, chartValueName)
		}

		// Generate helm templates to pass to gitops engine
		addedConnectStrings := helm.InstallOrUpgradeChart(helm.ChartOptions{
			BasePath:  componentPath.base,
			Chart:     chart,
			Component: component,
		})

		// Iterate over any connectStrings and add to the main map
		for name, description := range addedConnectStrings {
			connectStrings[name] = description
		}
	}

	for _, manifest := range component.Manifests {
		for idx := range manifest.Kustomizations {
			// Move kustomizations to files now
			destination := fmt.Sprintf("kustomization-%s-%d.yaml", manifest.Name, idx)
			manifest.Files = append(manifest.Files, destination)
		}

		// Iterate over any connectStrings and add to the main map
		for name, description := range helm.GenerateChart(componentPath.manifests, manifest, component) {
			connectStrings[name] = description
		}
	}

	for _, script := range component.Scripts.After {
		loopScriptUntilSuccess(script, component.Scripts)
	}

	if isSeedRegistry {
		postSeedRegistry(tempPath)
	}
}

// Wait for the target pod(s) to come up and inject the data into them
// todo:  this currently requires kubectl but we should have enough k8s work to make this native now
func handleDataInjection(wg *sync.WaitGroup, data types.ZarfDataInjection, componentPath componentPaths) {
	defer wg.Done()

	injectionCompletionMarker := componentPath.dataInjections + "/.zarf-sync-complete"
	if err := utils.WriteFile(injectionCompletionMarker, []byte("🦄")); err != nil {
		return
	}

	timeout := time.After(15 * time.Minute)
	for {
		// delay check 2 seconds
		time.Sleep(2 * time.Second)
		select {

		// on timeout abort
		case <-timeout:
			message.Warnf("data injection into target %s timed out\n", data.Target.Namespace)
			return

		default:
			sourceFile := componentPath.dataInjections + "/" + filepath.Base(data.Target.Path)

			// Wait until the pod we are injecting data into becomes available
			pods := k8s.WaitForPodsAndContainers(data.Target, true)
			if len(pods) < 1 {
				continue
			}

			// Define injection destination
			destination := data.Target.Path
			if destination == "/"+filepath.Base(destination) {
				// Handle top-level directory targets
				destination = "/"
			}

			// Inject into all the pods
			for _, pod := range pods {
				cpPodExecArgs := []string{"-n", data.Target.Namespace, "cp", sourceFile, pod + ":" + destination}

				if data.Target.Container != "" {
					// Append the container args if they are specified
					cpPodExecArgs = append(cpPodExecArgs, "-c", data.Target.Container)
				}

				// Do the actual data injection
				_, _, err := utils.ExecCommandWithContext(context.TODO(), true, "kubectl", cpPodExecArgs...)
				if err != nil {
					message.Warnf("Error copying data into the pod %#v: %#v\n", pod, err)
					continue
				} else {
					// Leave a marker in the target container for pods to track the sync action
					cpPodExecArgs[3] = injectionCompletionMarker
					cpPodExecArgs[4] = pod + ":" + data.Target.Path
					_, _, err = utils.ExecCommandWithContext(context.TODO(), true, "kubectl", cpPodExecArgs...)
					if err != nil {
						message.Warnf("Error saving the zarf sync completion file after injection into pod %#v\n", pod)
					}
				}
			}

			// Cleanup now to reduce disk pressure
			_ = os.RemoveAll(sourceFile)

			// Return to stop the loop
			return
		}
	}
}
