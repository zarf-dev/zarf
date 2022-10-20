package packager

import (
	"net/url"
	"os"
	"regexp"

	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"helm.sh/helm/v3/pkg/chart/loader"
)

func getTopLevelFiles(path string) (topLevelFiles []string) {
	dirContents, err := os.ReadDir(path)
	if err != nil {
		message.Fatal(err, "FS error")
	}
	for _, content := range dirContents {
		if !content.IsDir() {
			topLevelFiles = append(topLevelFiles, path + "/" + content.Name())
		}
	}
	return topLevelFiles
}

func isDir(path string) bool {
	pathData, err := os.Stat(path)
	if err != nil {
		message.Fatal(err, "FS error")
	}
	return pathData.IsDir()
}

func isLocalFiles(data string) bool {
	return false
}

func isGitChart(data string) bool {
	return false

}

func isHelmRepoChart(data string) bool {
	return false
}

func isLocalChart(data string) bool {
	_, err := loader.LoadDir(data)
	return err == nil
}

func isManifests(data string) bool {
	ManifestFiles = []string{}
	isYaml := regexp.MustCompile(`.*\.yaml$`).MatchString
	if isDir(data) {
		topLevelFiles := getTopLevelFiles(data)
		for _, topLevelFile := range topLevelFiles {
			if isYaml(topLevelFile) {
				ManifestFiles = append(ManifestFiles, topLevelFile)
			}
		}
		if len(ManifestFiles) > 0 {
			return true
		} else {
			return false
		}
	} else {
		if isYaml(data) {
			return true
		} else {
			return false
		}
	}
}

func isRemoteFile(data string) bool {
	return false
}

func isUrl(data string) bool {
	urlData, err := url.ParseRequestURI(data)
	return err == nil && urlData.Host != ""
}

var ManifestFiles []string


func DeduceResourceType(componentResource string) string {
	if !utils.InvalidPath(componentResource) {
		message.Info("Is path")
		if isLocalChart(componentResource) {
			return "localChart"
		} else if isManifests(componentResource) {
			return "manifests"
		} else if isLocalFiles(componentResource) {
			return "localFiles"
		} else {
			return "unknown path"
		}
	} else if isUrl(componentResource) {
		message.Info("Is url")
		if isGitChart(componentResource) {
			return "gitChart"
		} else if isHelmRepoChart(componentResource) {
			return "helmRepoChart"
		} else if isRemoteFile(componentResource) {
			return "remoteFile"
		} else {
			return "unknown url"
		}
	} else {
		return "unparsable"
	}

}