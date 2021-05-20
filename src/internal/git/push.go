package git

import (
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	log "github.com/sirupsen/logrus"
)

func Push(baseUrl string, path string) {

	logContext := log.WithField("repo", path)
	logContext.Info("Processing git repo")

	// Open the given repo
	repo, err := git.PlainOpen(path)
	if err != nil {
		logContext.Fatal("Unable to open the git repo")
	}

	// Get the orinin URL
	remote, err := repo.Remote("origin")
	if err != nil {
		logContext.Fatal("Unable to find the git remote")
	}
	remoteUrl := remote.Config().URLs[0]
	targetUrl, targetRepo  := transformURL(baseUrl, remoteUrl)

	_, _ = repo.CreateRemote(&config.RemoteConfig{
		Name: "utility",
		URLs: []string{targetUrl},
	})

	err = repo.Push(&git.PushOptions{
		RemoteName: "utility",
	})
	if err == git.NoErrAlreadyUpToDate {
		logContext.WithField("target", targetRepo).Info("Repo already up-to-date")
	} else if err != nil {
		logContext.WithField("target", targetRepo).Warn("Unable to push repo to the utility cluster")
	}

}
