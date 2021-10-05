package git

import (
	"github.com/go-git/go-git/v5"
	goConfig "github.com/go-git/go-git/v5/config"
	"github.com/sirupsen/logrus"
	"repo1.dso.mil/platform-one/big-bang/apps/product-tools/zarf/cli/config"
	"repo1.dso.mil/platform-one/big-bang/apps/product-tools/zarf/cli/internal/utils"
)

const offlineRemoteName = "offline-downstream"

func PushAllDirectories(localPath string) {
	paths := utils.ListDirectories(localPath)
	for _, path := range paths {
		push(path)
	}
}

func push(localPath string) {

	logContext := logrus.WithField("repo", localPath)
	logContext.Info("Processing git repo")

	// Open the given repo
	repo, err := git.PlainOpen(localPath)
	if err != nil {
		logContext.Warn("Not a valid git repo or unable to open")
		return
	}

	// Get the upstream URL
	remote, err := repo.Remote(onlineRemoteName)
	if err != nil {
		logContext.Warn("Unable to find the git remote")
		return
	}
	remoteUrl := remote.Config().URLs[0]
	targetUrl := transformURL("https://"+config.ZarfLocalIP, remoteUrl)

	_, _ = repo.CreateRemote(&goConfig.RemoteConfig{
		Name: offlineRemoteName,
		URLs: []string{targetUrl},
	})

	gitCred := FindAuthForHost(config.ZarfLocalIP)

	err = repo.Push(&git.PushOptions{
		RemoteName: offlineRemoteName,
		Auth:       &gitCred.Auth,
		RefSpecs: []goConfig.RefSpec{
			"refs/heads/*:refs/heads/*",
			"refs/tags/*:refs/tags/*",
		},
	})

	pushContext := logContext.WithField("target", targetUrl)
	if err == git.NoErrAlreadyUpToDate {
		pushContext.Info("Repo already up-to-date")
	} else if err != nil {
		pushContext.Warn("Unable to push repo to the gitops service")
	} else {
		pushContext.Info("Repo updated")
	}

}
