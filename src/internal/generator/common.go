package generator

import (
	"errors"
	"os"
	"regexp"

	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/defenseunicorns/zarf/src/types"
)

func GetPackageFromDestination(dest string) (generatePackage types.ZarfPackage, fileExists bool, computedDest string) {
	if dest == "" {
		computedDest = "zarf.yaml"
	} else {
		computedDest = dest
	}

	isYaml := regexp.MustCompile(`.*\.yaml$`).MatchString

	if isYaml(computedDest) {
		destInfo, err := os.Stat(computedDest)
		// Specified path exists
		if err == nil {
			// Specified path isn't a directory
			if destInfo.IsDir() {
				message.Fatal("", "The provided destination must not be a directory")
			}

			// Since path exists and isn't a dir read it
			err = utils.ReadYaml(computedDest, &generatePackage)
			if err != nil {
				message.Fatal(err, "Error parsing provided file.")
			}

			fileExists = true

		} else if errors.Is(err, os.ErrNotExist) {
			// Specified zarf file does not exist
			fileExists = false
			generatePackage = types.ZarfPackage{Kind: "ZarfPackageConfig"}

			} else {
				message.Fatalf(err, "Unknown error when loading specified file: %s", err.Error())
			}
	} else {
		message.Fatal("", "Path must be a \".yaml\" file")
	}

	return generatePackage, fileExists, computedDest
}
