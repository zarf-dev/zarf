package generator

import (
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"

	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"helm.sh/helm/v3/pkg/chart/loader"
)

func getTopLevelFiles(path string) (topLevelFiles []string) {
	if isDir(path) {
		dirContents, err := os.ReadDir(path)
		if err != nil {
			message.Fatal(err, "Error reading directory")
		}
		for _, content := range dirContents {
			topLevelFiles = append(topLevelFiles, filepath.Join(path, content.Name()))
		}
	} else {
		_, err := os.ReadFile(path)
		if err != nil {
			message.Fatal(err, "Error reading file")
		}
		topLevelFiles = append(topLevelFiles, path)
	}
	return topLevelFiles
}

func isLocalFiles(data string) bool {
	return len(getTopLevelFiles(data)) > 0
}

func isGitChart(data string) bool {
	isGitRepo := regexp.MustCompile(`\.git$`).MatchString
	return isGitRepo(data)
}

func isHelmRepoChart(data string) bool {
	indexReqUrl := data
	if (len(indexReqUrl) > 10 ) && (indexReqUrl[len(indexReqUrl)-10:] == "index.yaml") {
		message.Fatal("", "The url of a chart must not end with \"index.yaml\"")
	} else if indexReqUrl[len(indexReqUrl)-1:] == "/" {
		indexReqUrl = indexReqUrl + "index.yaml"
	} else {
		indexReqUrl = indexReqUrl + "/index.yaml"
	}
	response, err := http.Get(indexReqUrl)
	if err != nil {
		message.Fatal(err, err.Error())
	}
	message.Info(response.Status)
	return response.Status == "200 OK"
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
			if isYaml(topLevelFile) && isValidManifest(topLevelFile) {
				ManifestFiles = append(ManifestFiles, topLevelFile)
			}
		}
		if len(ManifestFiles) > 0 {
			return true
		} else {
			return false
		}
	} else {
		if isYaml(data) && isValidManifest(data) {
			return true
		} else {
			return false
		}
	}
}

func isRemoteFile(data string) bool {
	response, err := http.Get(data)
	if err != nil {
		message.Fatal(err, err.Error())
	}
	return response.Status == "200 OK"
}

func isUrl(data string) bool {
	urlData, err := url.ParseRequestURI(data)
	if err == nil && urlData.Host != "" {
		_, err = http.Get(data)
		if err != nil {
			message.Fatalf(err, "URL: %s is unreachable", data)
		} else {
			return true
		}
	}
	return false
}

func isValidManifest(path string) (isManifest bool) {
	var currentYaml yamlKind
		err := utils.ReadYaml(path, &currentYaml)
		if err != nil {
			message.Fatalf(err, "Error reading manifest %s", path)
		}
	message.Info(currentYaml.Kind)
	return currentYaml.Kind != ""
}

var ManifestFiles []string

func DeduceResourceType(componentResource string) string {
	if !utils.InvalidPath(componentResource) {
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
