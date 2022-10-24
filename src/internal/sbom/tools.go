package sbom

import (
	"os"
	"path/filepath"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
)

// WriteSBOMFiles 
func WriteSBOMFiles(sbomViewFiles []string) error {
	// Check if we even have any SBOM files to process
	if len(sbomViewFiles) == 0 {
		return nil
	}

	// Cleanup any failed prior removals
	_ = os.RemoveAll(config.ZarfSBOMDir)

	// Create the directory again
	err := utils.CreateDirectory(config.ZarfSBOMDir, 0755)
	if err != nil {
		return err
	}

	// Write each of the sbom files
	for _, file := range sbomViewFiles {
		// Our file copy lib explodes on these files for some reason...
		data, err := os.ReadFile(file)
		if err != nil {
			message.Fatalf(err, "Unable to read the sbom-viewer file %s", file)
		}
		dst := filepath.Join(config.ZarfSBOMDir, filepath.Base(file))
		err = os.WriteFile(dst, data, 0644)
		if err != nil {
			message.Debugf("Unable to write the sbom-viewer file %s", dst)
			return err
		}
	}

	return nil
}
