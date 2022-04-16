package utils

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"

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
	if strings.HasPrefix(url, SGETProtocol) {
		sgetFile(url, destinationFile, cosignKeyPath)
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
	// Remove the custom protocol header from the url
	_, url, _ = strings.Cut(url, SGETProtocol)

	// Override the stdout and stderr because the sget function prints directly to both
	oldStdout := os.Stdout
	oldStderr := os.Stderr
	readOut, writeOut, _ := os.Pipe()
	readErr, writeErr, _ := os.Pipe()
	os.Stdout = writeOut
	os.Stderr = writeErr
	defer writeOut.Close()
	defer writeErr.Close()

	// Use Cosign sget to verify and download the resource
	// NOTE: We are redirecting the output of the sget call from stdout to our debug logger
	err := sget.New(url, cosignKeyPath, destinationFile).Do(context.TODO())
	// print out the output of the sget as debug logs (as opposed to printed directly to stdout)
	writeOut.Close()
	writeErr.Close()
	output, _ := ioutil.ReadAll(readOut)
	errOutput, _ := ioutil.ReadAll(readErr)
	message.Debugf("sget stdout: %v\n", string(output))
	message.Debugf("sget stderr: %v\n", string(errOutput))
	if err != nil {
		message.Fatalf(err, "Unable to download file with sget: %v\n", url)
	}

	// Replace the original stdout and stderr
	os.Stdout = oldStdout
	os.Stderr = oldStderr
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
