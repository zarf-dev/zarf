package git

import (
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	log "github.com/sirupsen/logrus"
	"repo1.dso.mil/platform-one/big-bang/apps/product-tools/zarf/cli/internal/utils"
)

const remoteName = "zarf-target"

func PushAllDirectories(baseUrl string, path string) {
	paths := utils.ListDirectories(path)
	for _, entry := range paths {
		Push(baseUrl, entry)
	}
}

func Push(baseUrl string, path string) {

	logContext := log.WithField("repo", path)
	logContext.Info("Processing git repo")

	// Open the given repo
	repo, err := git.PlainOpen(path)
	if err != nil {
		logContext.Warn("Not a valid git repo or unable to open")
		return
	}

	// Get the orinin URL
	remote, err := repo.Remote("origin")
	if err != nil {
		logContext.Warn("Unable to find the git remote")
		return
	}
	remoteUrl := remote.Config().URLs[0]
	targetUrl, targetRepo := transformURL(baseUrl, remoteUrl)

	// Silently purge a remote if it already exists
	_ = repo.DeleteRemote(remoteName)

	_, _ = repo.CreateRemote(&config.RemoteConfig{
		Name: remoteName,
		URLs: []string{targetUrl},
	})

	err = repo.Push(&git.PushOptions{
		RemoteName: remoteName,
	})

	pushContext := logContext.WithField("target", targetRepo)
	if err == git.NoErrAlreadyUpToDate {
		pushContext.Info("Repo already up-to-date")
	} else if err != nil {
		pushContext.Warn("Unable to push repo to the utility cluster")
	} else {
		pushContext.Info("Repo updated")
	}

	// Silently purge the remote on completion
	_ = repo.DeleteRemote(remoteName)
}
