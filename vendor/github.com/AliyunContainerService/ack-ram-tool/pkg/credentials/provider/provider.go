package provider

import (
	"context"
	"fmt"
	"os"
	"path"
	"runtime"
)

var UserAgent = ""

type CredentialsProvider interface {
	Credentials(ctx context.Context) (*Credentials, error)
}

type Stopper interface {
	Stop(ctx context.Context)
}

func init() {
	name := path.Base(os.Args[0])
	UserAgent = fmt.Sprintf("%s %s/%s ack-ram-tool/provider/%s", name, runtime.GOOS, runtime.GOARCH, runtime.Version())
}
