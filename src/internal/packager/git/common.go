package git

import (
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/types"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
)

type Git struct {
	server types.GitServerInfo

	spinner *message.Spinner

	// Target working directory for the git repository
	gitPath string
}

type Credential struct {
	Path string
	Auth http.BasicAuth
}

const onlineRemoteName = "online-upstream"
const offlineRemoteName = "offline-downstream"
const onlineRemoteRefPrefix = "refs/remotes/" + onlineRemoteName + "/"

func New(server types.GitServerInfo) *Git {
	return &Git{
		server: server,
	}
}

func NewWithSpinner(server types.GitServerInfo, spinner *message.Spinner) *Git {
	return &Git{
		server:  server,
		spinner: spinner,
	}
}
