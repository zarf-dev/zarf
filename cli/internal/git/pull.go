package git

import (
	"os"

	"github.com/go-git/go-git/v5"
	"github.com/sirupsen/logrus"
)

const onlineRemoteName = "online-upstream"

func Pull(gitUrl string, targetFolder string, tag string) {

	logContext := logrus.WithFields(logrus.Fields{
		"Remote": gitUrl,
		"Tag":    tag,
	})
	logContext.Info("Processing git repo")

	gitCred := findAuthForHost(gitUrl)

	cloneOptions := &git.CloneOptions{
		Auth:          &gitCred.auth,
		URL:           gitUrl,
		Progress:      os.Stdout,
		RemoteName:    onlineRemoteName,
	}

	path := targetFolder + transformURLtoRepoName(gitUrl)

	// Clone the given repo
	_, err := git.PlainClone(path, false, cloneOptions)

	if err == git.ErrRepositoryAlreadyExists {
		logContext.Info("Repo already cloned")
	} else if err != nil {
		logContext.Warn("Not a valid git repo or unable to clone")
		return
	}

	logContext.Info("Git repo synced")

}
