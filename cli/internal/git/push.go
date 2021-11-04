package git

import (
	"os"

	"github.com/defenseunicorns/zarf/cli/config"
	"github.com/defenseunicorns/zarf/cli/internal/utils"
	"github.com/go-git/go-git/v5"
	goConfig "github.com/go-git/go-git/v5/config"
	"github.com/sirupsen/logrus"
)

const offlineRemoteName = "offline-downstream"
const onlineRemoteRefPrefix = "refs/remotes/" + onlineRemoteName + "/"

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
		logContext.Fatal("Not a valid git repo or unable to open")
	}

	// Get the upstream URL
	remote, err := repo.Remote(onlineRemoteName)
	if err != nil {
		logContext.Warn("Unable to find the git remote")
		return
	}
	remoteUrl := remote.Config().URLs[0]
	targetUrl := transformURL("https://"+config.ZarfLocalIP, remoteUrl)

	_, err = repo.CreateRemote(&goConfig.RemoteConfig{
		Name: offlineRemoteName,
		URLs: []string{targetUrl},
	})

	if err != nil {
		logContext.Debug(err)
		logContext.Fatal("Failed to create offline remote")
	}

	gitCred := FindAuthForHost(config.ZarfLocalIP)

	pushContext := logContext.WithField("target", targetUrl)

	// Since we are pushing HEAD:refs/heads/master on deployment, leaving
	// duplicates of the HEAD ref (ex. refs/heads/master,
	// refs/remotes/online-upstream/master, will cause the push to fail)
	removedRefs := RemoveHeadCopies(localPath)

	err = repo.Push(&git.PushOptions{
		RemoteName: offlineRemoteName,
		Auth:       &gitCred.Auth,
		Progress:   os.Stdout,
		// If a provided refspec doesn't push anything, it is just ignored
		RefSpecs: []goConfig.RefSpec{
			"refs/heads/*:refs/heads/*",
			onlineRemoteRefPrefix + "*:refs/heads/*",
			"refs/tags/*:refs/tags/*",
		},
	})

	if err == git.NoErrAlreadyUpToDate {
		pushContext.Info("Repo already up-to-date")
	} else if err != nil {
		pushContext.Debug(err)
		pushContext.Warn("Unable to push repo to the gitops service")
	} else {
		pushContext.Info("Repo updated")
	}

	// Add back the refs we removed just incase this push isn't the last thing
	// being run and a later task needs to reference them.
	AddRefs(localPath, removedRefs)
}
