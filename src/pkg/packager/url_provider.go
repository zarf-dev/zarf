// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package packager contains functions for interacting with, managing and deploying Zarf packages.
package packager

// // Handle case where deploying remote package validated via sget
// if strings.HasPrefix(p.cfg.PkgOpts.PackagePath, utils.SGETURLPrefix) {
// 	return partialPaths, p.handleSgetPackage()
// }

// // If packagePath has partial in the name, we need to combine the partials into a single package
// if err := p.handleIfPartialPkg(); err != nil {
// 	return fmt.Errorf("unable to process partial package: %w", err)
// }

// func (p *Packager) handleSgetPackage() error {
// 	message.Warn(lang.WarnSGetDeprecation)

// 	spinner := message.NewProgressSpinner("Loading Zarf Package %s", p.cfg.PkgOpts.PackagePath)
// 	defer spinner.Stop()

// 	// Create the local file for the package
// 	localPath := filepath.Join(p.tmp.Base, "remote.tar.zst")
// 	destinationFile, err := os.Create(localPath)
// 	if err != nil {
// 		return fmt.Errorf("unable to create the destination file: %w", err)
// 	}
// 	defer destinationFile.Close()

// 	// If this is a DefenseUnicorns package, use an internal sget public key
// 	if strings.HasPrefix(p.cfg.PkgOpts.PackagePath, fmt.Sprintf("%s://defenseunicorns", utils.SGETURLScheme)) {
// 		os.Setenv("DU_SGET_KEY", config.CosignPublicKey)
// 		p.cfg.PkgOpts.SGetKeyPath = "env://DU_SGET_KEY"
// 	}

// 	// Sget the package
// 	err = utils.Sget(context.TODO(), p.cfg.PkgOpts.PackagePath, p.cfg.PkgOpts.SGetKeyPath, destinationFile)
// 	if err != nil {
// 		return fmt.Errorf("unable to get the remote package via sget: %w", err)
// 	}

// 	p.cfg.PkgOpts.PackagePath = localPath

// 	spinner.Success()
// 	return nil
// }

type httpProvider struct {
	src      string
	dst      string
	shasum   string
	insecure bool
	signatureValidator
}

func (hp *httpProvider) LoadPackage(optionalComponents []string) ([]string, error) {
	// packageURL = fmt.Sprintf("%s@%s", p.cfg.PkgOpts.PackagePath, p.cfg.PkgOpts.Shasum)
	// if !config.CommonOptions.Insecure && p.cfg.PkgOpts.Shasum == "" {
	// 	return partialPaths, fmt.Errorf("remote package provided without a shasum, use --insecure to ignore")
	// }
	// message.Debug("Identified source as HTTPS")
	// tmp, err := utils.MakeTempDir()
	// if err != nil {
	// 	return nil, err
	// }
	// defer os.RemoveAll(tmp)
	// if err := utils.DownloadToFile(source, tmp, ""); err != nil {
	// 	return nil, err
	// }
	// if err := archiver.Unarchive(source, destination); err != nil {
	// 	return nil, err
	// }
	return nil, nil
}
