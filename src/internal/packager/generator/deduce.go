package generator

import (
	"net/http"
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
			lastCharIsForwardSlash := regexp.MustCompile(`\/$`).MatchString
			if lastCharIsForwardSlash(path) {
				topLevelFiles = append(topLevelFiles, path + content.Name())
			} else {
				topLevelFiles = append(topLevelFiles, path + "/" + content.Name())
			}
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
	if len(getTopLevelFiles(data)) > 0 {
		return true
	} else {
		return false
	}
}

func isGitChart(data string) bool {
	isGitRepo := regexp.MustCompile(`\.git$`).MatchString
	return isGitRepo(data)
}

func isHelmRepoChart(data string) bool {
	indexReqUrl := data
	if (len(indexReqUrl) > 10 ) && (indexReqUrl[len(indexReqUrl)-10:] == "index.yaml") {
		//already exact url which is weird but requires no modification
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
