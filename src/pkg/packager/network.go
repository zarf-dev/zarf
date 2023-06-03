package packager

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

// handlePackagePath If provided package is a URL download it to a temp directory.
func (p *Packager) handlePackagePath() error {
	message.Debug("packager.handlePackagePath()")

	opts := p.cfg.DeployOpts

	// Check if the user gave us a remote package
	providedURL, err := url.Parse(opts.PackagePath)
	if err != nil || providedURL.Scheme == "" || providedURL.Host == "" {
		message.Debug("Provided package path is not a URL, skipping download")
		return nil
	}

	// Handle case where deploying remote package stored in an OCI registry
	if utils.IsOCIURL(opts.PackagePath) {
		ociURL := opts.PackagePath
		p.cfg.DeployOpts.PackagePath = p.tmp.Base
		client, err := utils.NewOrasRemote(ociURL)
		if err != nil {
			return err
		}
		requestedComponents := getRequestedComponentList(p.cfg.DeployOpts.Components)
		layersToPull := []ocispec.Descriptor{}
		if len(requestedComponents) > 0 {
			layers, err := client.CalculateLayersToPullFromRequestedComponents(requestedComponents)
			if err != nil {
				return fmt.Errorf("unable to get published component image layers: %s", err.Error())
			}
			layersToPull = append(layersToPull, layers...)
		}
		return client.PullPackage(p.tmp.Base, p.cfg.PublishOpts.CopyOptions.Concurrency, layersToPull...)
	}

	// Handle case where deploying remote package validated via sget
	if strings.HasPrefix(opts.PackagePath, utils.SGETURLPrefix) {
		return p.handleSgetPackage()
	}

	spinner := message.NewProgressSpinner("Loading Zarf Package %s", opts.PackagePath)
	defer spinner.Stop()

	if !config.CommonOptions.Insecure && opts.Shasum == "" {
		return fmt.Errorf("remote package provided without a shasum, use --insecure to ignore")
	}

	// Check the extension on the package is what we expect
	if !isValidFileExtension(providedURL.Path) {
		return fmt.Errorf("remote package provided with an invalid extension, must be one of: %s", config.GetValidPackageExtensions())
	}

	// Download the package
	resp, err := http.Get(opts.PackagePath)
	if err != nil {
		return fmt.Errorf("unable to download remote package: %w", err)
	}
	defer resp.Body.Close()

	localPath := p.tmp.Base + providedURL.Path
	message.Debugf("Creating local package with the path: %s", localPath)
	packageFile, _ := os.Create(localPath)
	_, err = io.Copy(packageFile, resp.Body)
	if err != nil {
		return fmt.Errorf("unable to copy the contents of the provided URL into a local file: %w", err)
	}

	// Check the shasum if necessary
	if !config.CommonOptions.Insecure {
		hasher := sha256.New()
		_, err = io.Copy(hasher, packageFile)
		if err != nil {
			return fmt.Errorf("unable to calculate the sha256 of the provided remote package: %w", err)
		}

		value := hex.EncodeToString(hasher.Sum(nil))
		if value != opts.Shasum {
			_ = os.Remove(localPath)
			return fmt.Errorf("shasum of remote package does not match provided shasum, expected %s, got %s", opts.Shasum, value)
		}
	}

	opts.PackagePath = localPath

	spinner.Success()
	return nil
}

func (p *Packager) handleSgetPackage() error {
	message.Debug("packager.handleSgetPackage()")

	opts := p.cfg.DeployOpts

	spinner := message.NewProgressSpinner("Loading Zarf Package %s", opts.PackagePath)
	defer spinner.Stop()

	// Create the local file for the package
	localPath := filepath.Join(p.tmp.Base, "remote.tar.zst")
	destinationFile, err := os.Create(localPath)
	if err != nil {
		return fmt.Errorf("unable to create the destination file: %w", err)
	}
	defer destinationFile.Close()

	// If this is a DefenseUnicorns package, use an internal sget public key
	if strings.HasPrefix(opts.PackagePath, fmt.Sprintf("%s://defenseunicorns", utils.SGETURLScheme)) {
		os.Setenv("DU_SGET_KEY", config.SGetPublicKey)
		p.cfg.DeployOpts.SGetKeyPath = "env://DU_SGET_KEY"
	}

	// Sget the package
	err = utils.Sget(context.TODO(), opts.PackagePath, p.cfg.DeployOpts.SGetKeyPath, destinationFile)
	if err != nil {
		return fmt.Errorf("unable to get the remote package via sget: %w", err)
	}

	p.cfg.DeployOpts.PackagePath = localPath

	spinner.Success()
	return nil
}
