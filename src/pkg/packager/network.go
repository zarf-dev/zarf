package packager

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
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
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content/file"
	"oras.land/oras-go/v2/registry"
	"oras.land/oras-go/v2/registry/remote/errcode"
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
	ref, err := registry.ParseReference(strings.TrimPrefix(p.cfg.DeployOpts.PackagePath, "oci://"))
	if err != nil {
		return fmt.Errorf("failed to parse OCI reference: %w", err)
	}

	out := p.tmp.Base
	message.Debugf("Pulling %s", ref.String())
	message.Infof("Pulling Zarf package from %s", ref)
	spinner := message.NewProgressSpinner("")
	defer spinner.Stop()

	repo, ctx, err := utils.OrasRemote(ref)
	if err != nil {
		return err
	}

	copyOpts := oras.DefaultCopyOptions
	copyOpts.Concurrency = p.cfg.PublishOpts.CopyOptions.Concurrency
	spinner.Updatef("Pulling %d layers concurrently", copyOpts.Concurrency)
	copyOpts.OnCopySkipped = func(ctx context.Context, desc ocispec.Descriptor) error {
		title := desc.Annotations[ocispec.AnnotationTitle]
		var format string
		if title != "" {
			format = fmt.Sprintf("%s %s", desc.Digest.Encoded()[:12], utils.First30last30(title))
		} else {
			format = fmt.Sprintf("%s [%s]", desc.Digest.Encoded()[:12], desc.MediaType)
		}
		message.Successf(format)
		return nil
	}
	copyOpts.PostCopy = copyOpts.OnCopySkipped

	dst, err := file.New(out)
	if err != nil {
		return err
	}
	defer dst.Close()

	_, err = oras.Copy(ctx, repo, ref.Reference, dst, ref.Reference, copyOpts)
	if err != nil {
		return err
	}

	message.Debugf("Pulled %s", ref.String())
	message.Successf("Pulled %s", ref.String())

	p.cfg.DeployOpts.PackagePath = out
	return nil
}

// isManifestUnsupported returns true if the error is an unsupported artifact manifest error.
//
// This function was copied verbatim from https://github.com/oras-project/oras/blob/main/cmd/oras/push.go
func isManifestUnsupported(err error) bool {
	var errResp *errcode.ErrorResponse
	if !errors.As(err, &errResp) || errResp.StatusCode != http.StatusBadRequest {
		return false
	}

	var errCode errcode.Error
	if !errors.As(errResp, &errCode) {
		return false
	}

	// As of November 2022, ECR is known to return UNSUPPORTED error when
	// putting an OCI artifact manifest.
	switch errCode.Code {
	case errcode.ErrorCodeManifestInvalid, errcode.ErrorCodeUnsupported:
		return true
	}
	return false
}
