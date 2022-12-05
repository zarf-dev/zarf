package live

import (
	"io"
	"net/http"
	"os"

	"os/exec"

	"github.com/pterm/pterm"
)

func StreamPterm(w http.ResponseWriter, r *http.Request) {
	// not really a stream
	read, write := io.Pipe()
	multi := io.MultiWriter(os.Stderr, write)
	pterm.SetDefaultOutput(multi)
	go io.Copy(w, read)
	pterm.Info.Println("Hello World!")
	write.Write([]byte(""))
	write.Close()
}

func StreamCmd(w http.ResponseWriter, r *http.Request) {
	cmd := exec.Command("ls")
	read, write := io.Pipe()
	cmd.Stdout = write
	cmd.Stderr = write
	go io.Copy(w, read)
	cmd.Run()
	write.Close()
}
