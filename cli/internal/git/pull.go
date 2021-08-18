package git

import (
	"os"

	"github.com/go-git/go-git/v5"
	"github.com/sirupsen/logrus"
	"repo1.dso.mil/platform-one/big-bang/apps/product-tools/zarf/cli/internal/utils"
)

const onlineRemoteName = "online-upstream"

func DownloadRepoToTemp(gitUrl string) string {
	path := utils.MakeTempDir()
	pull(gitUrl, path)
	return path
}

func Pull(gitUrl string, targetFolder string) {
	path := targetFolder + "/" + transformURLtoRepoName(gitUrl)
	pull(gitUrl, path)
}

func pull(gitUrl string, targetFolder string) {
	logContext := logrus.WithFields(logrus.Fields{
		"Remote": gitUrl,
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
		logContext.Fatal("Not a valid git repo or unable to clone")
	}

	logContext.Info("Git repo synced")
}
