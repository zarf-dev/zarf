package git

import (
	"fmt"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/internal/k8s"
	"github.com/defenseunicorns/zarf/src/internal/message"
	"github.com/defenseunicorns/zarf/src/internal/utils"
	"github.com/go-git/go-git/v5"
	goConfig "github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
)

const offlineRemoteName = "offline-downstream"
const onlineRemoteRefPrefix = "refs/remotes/" + onlineRemoteName + "/"

func PushAllDirectories(localPath string) {
	// Establish a git tunnel to send the repos
	tunnel := k8s.NewZarfTunnel()
	tunnel.Connect(k8s.ZarfGit, false)

	paths, err := utils.ListDirectories(localPath)
	if err != nil {
		message.Fatalf(err, "unable to list the %s directory", localPath)
	}

	spinner := message.NewProgressSpinner("Processing %d git repos", len(paths))
	defer spinner.Stop()

	for _, path := range paths {
		spinner.Updatef("Pushing git repo %s", localPath)
		if err := push(path, spinner); err != nil {
			spinner.Fatalf(err, "Unable to push the git repo %s", localPath)
		}
	}

	spinner.Success()
	tunnel.Close()
}

func push(localPath string, spinner *message.Spinner) error {

	// Open the given repo
	repo, err := git.PlainOpen(localPath)
	if err != nil {
		return fmt.Errorf("not a valid git repo or unable to open: %w", err)
	}

	// Get the upstream URL
	remote, err := repo.Remote(onlineRemoteName)
	if err != nil {
		return fmt.Errorf("unable to find the git remote: %w", err)

	}
	remoteUrl := remote.Config().URLs[0]
	targetHost := fmt.Sprintf("http://%s:%d", config.IPV4Localhost, k8s.PortGit)
	targetUrl := transformURL(targetHost, remoteUrl)

	_, err = repo.CreateRemote(&goConfig.RemoteConfig{
		Name: offlineRemoteName,
		URLs: []string{targetUrl},
	})

	if err != nil {
		return fmt.Errorf("failed to create offline remote: %w", err)
	}

	gitCred := http.BasicAuth{
		Username: config.ZarfGitPushUser,
		Password: config.GetSecret(config.StateGitPush),
	}

	// Since we are pushing HEAD:refs/heads/master on deployment, leaving
	// duplicates of the HEAD ref (ex. refs/heads/master,
	// refs/remotes/online-upstream/master, will cause the push to fail)
	removedRefs, err := removeHeadCopies(localPath)
	if err != nil {
		return fmt.Errorf("unable to remove unused git refs from the repo: %w", err)
	}

	err = repo.Push(&git.PushOptions{
		RemoteName: offlineRemoteName,
		Auth:       &gitCred,
		Progress:   spinner,
		// If a provided refspec doesn't push anything, it is just ignored
		RefSpecs: []goConfig.RefSpec{
			"refs/heads/*:refs/heads/*",
			onlineRemoteRefPrefix + "*:refs/heads/*",
			"refs/tags/*:refs/tags/*",
		},
	})

	if err == git.NoErrAlreadyUpToDate {
		spinner.Debugf("Repo already up-to-date")
	} else if err != nil {
		return fmt.Errorf("unable to push repo to the gitops service: %w", err)
	}

	// Add back the refs we removed just incase this push isn't the last thing
	// being run and a later task needs to reference them.
	addRefs(localPath, removedRefs)

	return nil
}
