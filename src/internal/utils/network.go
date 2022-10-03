package utils

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"

	"github.com/defenseunicorns/zarf/src/internal/message"
)

const SGETProtocol = "sget://"

func IsUrl(source string) bool {
	parsedUrl, err := url.Parse(source)
	return err == nil && parsedUrl.Scheme != "" && parsedUrl.Host != ""
}

// DoesHostnamesMatch returns a boolean indicating if the hostname of two different URLs are the same.
func DoesHostnamesMatch(url1 string, url2 string) (bool, error) {
	parsedURL1, err := url.Parse(url1)
	if err != nil {
		message.Debugf("unable to parse the url (%s)", url1)

		return false, err
	}
	parsedURL2, err := url.Parse(url2)
	if err != nil {
		message.Debugf("unable to parse the url (%s)", url2)

		return false, err
	}

	return parsedURL1.Hostname() == parsedURL2.Hostname(), nil
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
	title := fmt.Sprintf("Downloading %s", path.Base(url))
	progressBar := message.NewProgressBar(resp.ContentLength, title)

	if _, err = io.Copy(destinationFile, io.TeeReader(resp.Body, progressBar)); err != nil {
		progressBar.Fatalf(err, "Unable to save the file %s", destinationFile.Name())
	}

	progressBar.Success(text)
}

func sgetFile(url string, destinationFile *os.File, cosignKeyPath string) {
	// Remove the custom protocol header from the url
	_, url, _ = strings.Cut(url, SGETProtocol)
	err := Sget(url, cosignKeyPath, destinationFile, context.TODO())
	if err != nil {
		message.Fatalf(err, "Unable to download file with sget: %s\n", url)
	}
}
