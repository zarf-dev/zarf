package utils

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"

	"github.com/defenseunicorns/zarf/src/internal/message"
	"github.com/pterm/pterm"
	"github.com/sigstore/cosign/pkg/sget"
)

const SGETProtocol = "sget://"

func IsUrl(source string) bool {
	parsedUrl, err := url.Parse(source)
	return err == nil && parsedUrl.Scheme != "" && parsedUrl.Host != ""
}

func Fetch(url string) io.ReadCloser {
	// Get the data
	resp, err := http.Get(url)
	if err != nil {
		message.Fatal(err, "Unable to download the file")
	}

	// Check server response
	if resp.StatusCode != http.StatusOK {
		message.Fatalf(nil, "Bad HTTP status: %s", resp.Status)
	}

	return resp.Body
}

func DownloadToFile(url string, target string, cosignKeyPath string) {

	// Create the file
	destinationFile, err := os.Create(target)
	if err != nil {
		message.Fatal(err, "Unable to create the destination file")
	}
	defer destinationFile.Close()

	// If the url start with the sget protocol use that, otherwise do a typical GET call
	if url[:len(SGETProtocol)] == SGETProtocol {
		sgetFile(url[len(SGETProtocol):], destinationFile, cosignKeyPath)
	} else {
		httpGetFile(url, destinationFile)
	}
}

func httpGetFile(url string, destinationFile *os.File) {
	// Get the data
	resp, err := http.Get(url)
	if err != nil {
		message.Fatal(err, "Unable to download the file")
	}
	defer resp.Body.Close()

	// Check server response
	if resp.StatusCode != http.StatusOK {
		message.Fatalf(nil, "Bad HTTP status: %s", resp.Status)
	}

	// Writer the body to file
	text := fmt.Sprintf("Downloading %s", url)
	counter := NewWriteCounter(url, int(resp.ContentLength))

	if _, err = io.Copy(destinationFile, io.TeeReader(resp.Body, counter)); err != nil {
		_, _ = counter.progress.Stop()
		message.Fatalf(err, "Unable to save the file %s", destinationFile.Name())
	}

	_, _ = counter.progress.Stop()
	pterm.Success.Println(text)
}

func sgetFile(url string, destinationFile *os.File, cosignKeyPath string) {
	// Get the data
	err := sget.New(url, cosignKeyPath, destinationFile).Do(context.TODO())
	if err != nil {
		message.Fatalf(err, "Unable to sget the file %v", url)
	}

	return
}

type WriteCounter struct {
	Total    int
	progress *pterm.ProgressbarPrinter
}

func NewWriteCounter(url string, total int) *WriteCounter {
	// keep it brief to avoid a panic on smaller windows
	title := fmt.Sprintf("Downloading %s", path.Base(url))
	if total < 1 {
		message.Debugf("invalid content length detected: %v", total)
	}
	progressBar, _ := pterm.DefaultProgressbar.
		WithTotal(total).
		WithShowCount(false).
		WithTitle(title).
		WithRemoveWhenDone(true).
		Start()
	return &WriteCounter{
		Total:    total,
		progress: progressBar,
	}
}

func (wc *WriteCounter) Write(p []byte) (int, error) {
	n := len(p)
	wc.progress.Add(n)
	return n, nil
}
