package squashfs

import (
	"io"
	"io/fs"
	"runtime"

	"github.com/sylabs/squashfs/internal/routinemanager"
)

type ExtractionOptions struct {
	manager            *routinemanager.Manager
	LogOutput          io.Writer   //Where the verbose log should write.
	DereferenceSymlink bool        //Replace symlinks with the target file.
	UnbreakSymlink     bool        //Try to make sure symlinks remain unbroken when extracted, without changing the symlink.
	Verbose            bool        //Prints extra info to log on an error.
	IgnorePerm         bool        //Ignore file's permissions and instead use Perm.
	Perm               fs.FileMode //Permission to use when IgnorePerm. Defaults to 0777.
	SimultaneousFiles  uint16      //Number of files to process in parallel. Default set based on runtime.NumCPU().
	ExtractionRoutines uint16      //Number of goroutines to use for each file's extraction. Only applies to regular files. Default set based on runtime.NumCPU().
}

// The default extraction options.
func DefaultOptions() *ExtractionOptions {
	cores := uint16(runtime.NumCPU() / 2)
	var files, routines uint16
	if cores <= 4 {
		files = 1
		routines = cores
	} else {
		files = cores - 4
		routines = 4
	}
	return &ExtractionOptions{
		Perm:               0777,
		SimultaneousFiles:  files,
		ExtractionRoutines: routines,
	}
}

// Less limited default options. Can run up 2x faster than DefaultOptions.
// Tends to use all available CPU resources.
func FastOptions() *ExtractionOptions {
	return &ExtractionOptions{
		Perm:               0777,
		SimultaneousFiles:  uint16(runtime.NumCPU()),
		ExtractionRoutines: uint16(runtime.NumCPU()),
	}
}
