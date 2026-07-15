package file

import "io"

type Opener func() (io.ReadCloser, error)
