package git

import (
	"errors"
	"fmt"
	"path/filepath"

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

func PushAllDirectories(localPath string) error {
	gitServerInfo := config.GetGitServerInfo()
	gitServerURL := gitServerInfo.Address

	// If this is a serviceURL, create a port-forward tunnel to that resource
	if k8s.IsServiceURL(gitServerURL) {
		tunnel, err := k8s.NewTunnelFromServiceURL(gitServerURL)
		if err != nil {
			return err
		}

		tunnel.Connect("", false)
		defer tunnel.Close()
		gitServerURL = fmt.Sprintf("http://%s", tunnel.Endpoint())
	}

	paths, err := utils.ListDirectories(localPath)
	if err != nil {
		message.Warnf("Unable to list the %s directory", localPath)
		return err
	}

	spinner := message.NewProgressSpinner("Processing %d git repos", len(paths))
	defer spinner.Stop()

	for _, path := range paths {
		basename := filepath.Base(path)
		spinner.Updatef("Pushing git repo %s", basename)

		repo, err := prepRepoForPush(path, gitServerURL, gitServerInfo.PushUsername)
		if err != nil {
			message.Warnf("error when preping the repo for push.. %v", err)
			return err
		}

		if err := push(repo, path, spinner); err != nil {
			spinner.Warnf("Unable to push the git repo %s", basename)
			return err
		}

		// Add the read-only user to this repo
		if gitServerInfo.InternalServer {
			// Get the upstream URL
			remote, err := repo.Remote(onlineRemoteName)
			if err != nil {
				message.Warn("unable to get the information needed to add the read-only user to the repo")
				return err
			}
			remoteUrl := remote.Config().URLs[0]
			repoName, err := transformURLtoRepoName(remoteUrl)
			if err != nil {
				message.Warnf("Unable to add the read-only user to the repo: %s\n", repoName)
				return err
			}

			err = addReadOnlyUserToRepo(gitServerURL, repoName)
			if err != nil {
				message.Warnf("Unable to add the read-only user to the repo: %s\n", repoName)
				return err
			}
		}
	}

	spinner.Success()
	return nil
}

func prepRepoForPush(localPath, tunnelUrl, username string) (*git.Repository, error) {
	// Open the given repo
	repo, err := git.PlainOpen(localPath)
	if err != nil {
		return nil, fmt.Errorf("not a valid git repo or unable to open: %w", err)
	}

	// Get the upstream URL
	remote, err := repo.Remote(onlineRemoteName)
	if err != nil {
		return nil, fmt.Errorf("unable to find the git remote: %w", err)
	}

	remoteUrl := remote.Config().URLs[0]
	targetUrl, err := transformURL(tunnelUrl, remoteUrl, username)
	if err != nil {
		return nil, fmt.Errorf("unable to transform the git url: %w", err)
	}

	_, err = repo.CreateRemote(&goConfig.RemoteConfig{
		Name: offlineRemoteName,
		URLs: []string{targetUrl},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create offline remote: %w", err)
	}

	return repo, nil
}

func push(repo *git.Repository, localPath string, spinner *message.Spinner) error {
	gitCred := http.BasicAuth{
		Username: config.GetState().GitServer.PushUsername,
		Password: config.GetState().GitServer.PushPassword,
	}

	// Since we are pushing HEAD:refs/heads/master on deployment, leaving
	// duplicates of the HEAD ref (ex. refs/heads/master,
	// refs/remotes/online-upstream/master, will cause the push to fail)
	removedRefs, err := removeHeadCopies(localPath)
	if err != nil {
		return fmt.Errorf("unable to remove unused git refs from the repo: %w", err)
	}

	// // Fetch remote offline refs in case of old update or if multiple refs are specified in one package
	// fetchOptions := &git.FetchOptions{
	// 	RemoteName: offlineRemoteName,
	// 	Auth:       &gitCred,
	// 	RefSpecs: []goConfig.RefSpec{
	// 		"refs/heads/*:refs/heads/*",
	// 		onlineRemoteRefPrefix + "*:refs/heads/*",
	// 		"refs/tags/*:refs/tags/*",
	// 	},
	// }

	// err = repo.Fetch(fetchOptions)
	// if errors.Is(err, transport.ErrRepositoryNotFound) {
	// 	message.Debugf("Repo not yet available offline, skipping fetch")
	// } else if err != nil {
	// 	return fmt.Errorf("unable to fetch remote cleanly prior to push: %w", err)
	// }

	// Push all heads and tags to the offline remote
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

	if errors.Is(err, git.NoErrAlreadyUpToDate) {
		spinner.Debugf("Repo already up-to-date")
	} else if err != nil {
		return fmt.Errorf("unable to push repo to the gitops service: %w", err)
	}

	// Add back the refs we removed just incase this push isn't the last thing
	// being run and a later task needs to reference them.
	addRefs(localPath, removedRefs)

	return nil
}
