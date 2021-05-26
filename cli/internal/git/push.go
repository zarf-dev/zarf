package git

import (
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	log "github.com/sirupsen/logrus"
	"repo1.dso.mil/platform-one/big-bang/apps/product-tools/shift/pack/cli/internal/utils"
)

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

	_, _ = repo.CreateRemote(&config.RemoteConfig{
		Name: "utility",
		URLs: []string{targetUrl},
	})

	err = repo.Push(&git.PushOptions{
		RemoteName: "utility",
	})

	pushContext := logContext.WithField("target", targetRepo)
	if err == git.NoErrAlreadyUpToDate {
		pushContext.Info("Repo already up-to-date")
	} else if err != nil {
		pushContext.Warn("Unable to push repo to the utility cluster")
	} else {
		pushContext.Info("Repo updated")
	}

}
