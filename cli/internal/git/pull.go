package git

import (
	"os"

	"github.com/go-git/go-git/v5"
	"github.com/sirupsen/logrus"
	"repo1.dso.mil/platform-one/big-bang/apps/product-tools/zarf/cli/internal/utils"
)

const onlineRemoteName = "online-upstream"

func DownloadRepoToTemp(gitUrl string, tag string) string {
	path := utils.MakeTempDir()
	success := pull(gitUrl, tag, path)
	if success {
		return path
	} else {
		return ""
	}
}

func Pull(gitUrl string, targetFolder string, tag string) {
	path := targetFolder + "/" + transformURLtoRepoName(gitUrl)
	pull(gitUrl, tag, path)
}

func pull(gitUrl string, tag string, targetFolder string) bool {
	logContext := logrus.WithFields(logrus.Fields{
		"Remote": gitUrl,
		"Tag":    tag,
	})
	logContext.Info("Processing git repo")

	gitCred := FindAuthForHost(gitUrl)

	cloneOptions := &git.CloneOptions{
		Auth:       &gitCred.Auth,
		URL:        gitUrl,
		Progress:   os.Stdout,
		RemoteName: onlineRemoteName,
	}

	// Clone the given repo
	_, err := git.PlainClone(targetFolder, false, cloneOptions)

	if err == git.ErrRepositoryAlreadyExists {
		logContext.Info("Repo already cloned")
	} else if err != nil {
		logContext.Warn("Not a valid git repo or unable to clone")
		return false
	}

	logContext.Info("Git repo synced")
	return true
}
