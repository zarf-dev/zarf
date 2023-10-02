package packager

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/config/lang"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/defenseunicorns/zarf/src/pkg/utils/helpers"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

// handlePackagePath If provided package is a URL download it to a temp directory.
func (p *Packager) handlePackagePath() (partialPaths []string, err error) {

	// Check if the user gave us a remote package
	providedURL, err := url.Parse(p.cfg.PkgOpts.PackagePath)
	if err != nil || providedURL.Scheme == "" || providedURL.Host == "" {
		message.Debug("Provided package path is not a URL, skipping download")
		return partialPaths, nil
	}

	// Handle case where deploying remote package stored in an OCI registry
	if helpers.IsOCIURL(p.cfg.PkgOpts.PackagePath) {
		p.cfg.PkgOpts.PackagePath = p.tmp.Base
		requestedComponents := getRequestedComponentList(p.cfg.PkgOpts.OptionalComponents)
		layersToPull := []ocispec.Descriptor{}
		// only pull specified components and their images if --components AND --confirm are set
		if len(requestedComponents) > 0 && config.CommonOptions.Confirm {
			layers, err := p.remote.LayersFromRequestedComponents(requestedComponents)
			if err != nil {
				return partialPaths, fmt.Errorf("unable to get published component image layers: %s", err.Error())
			}
			layersToPull = append(layersToPull, layers...)
		}

		return p.remote.PullPackage(p.tmp.Base, config.CommonOptions.OCIConcurrency, layersToPull...)
	}

	// Handle case where deploying remote package validated via sget
	if strings.HasPrefix(p.cfg.PkgOpts.PackagePath, helpers.SGETURLPrefix) {
		return partialPaths, p.handleSgetPackage()
	}

	spinner := message.NewProgressSpinner("Loading Zarf Package %s", p.cfg.PkgOpts.PackagePath)
	defer spinner.Stop()

	if !config.CommonOptions.Insecure && p.cfg.PkgOpts.Shasum == "" {
		return partialPaths, fmt.Errorf("remote package provided without a shasum, use --insecure to ignore")
	}

	// Check the extension on the package is what we expect
	if !isValidFileExtension(providedURL.Path) {
		return partialPaths, fmt.Errorf("remote package provided with an invalid extension, must be one of: %s", config.GetValidPackageExtensions())
	}

	localPath := p.tmp.Base + providedURL.Path
	message.Debugf("Downloading the local package with the path: %s", localPath)

	packageURL := p.cfg.PkgOpts.PackagePath

	if !config.CommonOptions.Insecure {
		packageURL = fmt.Sprintf("%s@%s", p.cfg.PkgOpts.PackagePath, p.cfg.PkgOpts.Shasum)
	}

	utils.DownloadToFile(packageURL, localPath, "")

	p.cfg.PkgOpts.PackagePath = localPath

	spinner.Success()
	return partialPaths, nil
}

func (p *Packager) handleSgetPackage() error {
	message.Warn(lang.WarnSGetDeprecation)

	spinner := message.NewProgressSpinner("Loading Zarf Package %s", p.cfg.PkgOpts.PackagePath)
	defer spinner.Stop()

	// Create the local file for the package
	localPath := filepath.Join(p.tmp.Base, "remote.tar.zst")
	destinationFile, err := os.Create(localPath)
	if err != nil {
		return fmt.Errorf("unable to create the destination file: %w", err)
	}
	defer destinationFile.Close()

	// If this is a DefenseUnicorns package, use an internal sget public key
	if strings.HasPrefix(p.cfg.PkgOpts.PackagePath, fmt.Sprintf("%s://defenseunicorns", helpers.SGETURLScheme)) {
		os.Setenv("DU_SGET_KEY", config.CosignPublicKey)
		p.cfg.PkgOpts.SGetKeyPath = "env://DU_SGET_KEY"
	}

	// Sget the package
	err = utils.Sget(context.TODO(), p.cfg.PkgOpts.PackagePath, p.cfg.PkgOpts.SGetKeyPath, destinationFile)
	if err != nil {
		return fmt.Errorf("unable to get the remote package via sget: %w", err)
	}

	p.cfg.PkgOpts.PackagePath = localPath

	spinner.Success()
	return nil
}
