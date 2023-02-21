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
	"github.com/google/go-containerregistry/pkg/name"
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

	// Handle case where deploying remote package validated via sget
	if strings.HasPrefix(opts.PackagePath, "sget://") {
		return p.handleSgetPackage()
	}

	// Handle case where deploying remote package stored in an OCI registry
	if strings.HasPrefix(opts.PackagePath, "oci://") {
		return p.handleOciPackage()
	}

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

	return nil
}

func (p *Packager) handleSgetPackage() error {
	message.Debug("packager.handleSgetPackage()")

	opts := p.cfg.DeployOpts

	// Create the local file for the package
	localPath := filepath.Join(p.tmp.Base, "remote.tar.zst")
	destinationFile, err := os.Create(localPath)
	if err != nil {
		return fmt.Errorf("unable to create the destination file: %w", err)
	}
	defer destinationFile.Close()

	// If this is a DefenseUnicorns package, use an internal sget public key
	if strings.HasPrefix(opts.PackagePath, "sget://defenseunicorns") {
		os.Setenv("DU_SGET_KEY", config.SGetPublicKey)
		p.cfg.DeployOpts.SGetKeyPath = "env://DU_SGET_KEY"
	}

	// Remove the 'sget://' header for the actual sget call
	remoteBlob := strings.TrimPrefix(opts.PackagePath, "sget://")

	// Sget the package
	err = utils.Sget(context.TODO(), remoteBlob, p.cfg.DeployOpts.SGetKeyPath, destinationFile)
	if err != nil {
		return fmt.Errorf("unable to get the remote package via sget: %w", err)
	}

	p.cfg.DeployOpts.PackagePath = localPath

	return nil
}

func (p *Packager) handleOciPackage() error {
	message.Debug("packager.handleOciPackage()")
	ref, err := name.ParseReference(strings.TrimPrefix(p.cfg.DeployOpts.PackagePath, "oci://"), name.StrictValidation)
	if err != nil {
		return fmt.Errorf("failed to parse OCI reference: %w", err)
	}
	// patch docker.io to registry-1.docker.io
	if ref.Context().RegistryStr() == "docker.io" {
		ref, err = name.ParseReference(fmt.Sprintf("registry-1.docker.io/%s", ref.Context().RepositoryStr()), name.StrictValidation)
		if err != nil {
			return fmt.Errorf("failed to parse OCI reference: %w", err)
		}
	}

	out := p.tmp.Base
	message.Debugf("Pulling %s", ref.String())
	spinner := message.NewProgressSpinner("")
	pullOpts := utils.PullOCIZarfPackageOpts{
		Outdir:  out,
		Ref:     ref,
		Spinner: spinner,
	}
	err = utils.PullOCIZarfPackage(pullOpts)
	if err != nil {
		return fmt.Errorf("failed to pull package from OCI: %w", err)
	}
	message.Debugf("Pulled %s", ref.String())
	spinner.Successf("Pulled %s", ref.String())

	p.cfg.DeployOpts.PackagePath = out
	return nil
}
