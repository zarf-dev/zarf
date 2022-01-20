package utils

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"

	"github.com/defenseunicorns/zarf/cli/internal/message"
	"github.com/pterm/pterm"
)

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

func DownloadToFile(url string, target string) {

	// Create the file
	destinationFile, err := os.Create(target)
	if err != nil {
		message.Fatal(err, "Unable to create the destination file")
	}
	defer destinationFile.Close()

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
		message.Fatalf(err, "Unable to save the file %s", target)
	}

	_, _ = counter.progress.Stop()
	pterm.Success.Println(text)
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
