package presenter

import "io"

type Presenter interface {
	Present(io.Writer) error
}