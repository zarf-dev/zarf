package cosign

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"strings"

	"github.com/defenseunicorns/zarf/src/pkg/message"
)

const SGETProtocol = "sget://"

func DownloadToFile(url string, target string, cosignKeyPath string) error {
	destinationFile, err := os.Create(target)
	if err != nil {
		return err
	}
	defer destinationFile.Close()

	// If the url start with the sget protocol use that, otherwise do a typical GET call
	if strings.HasPrefix(url, SGETProtocol) {
		sgetFile(url, destinationFile, cosignKeyPath)
	} else {
		httpGetFile(url, destinationFile)
	}

	return nil
}

func httpGetFile(url string, destinationFile *os.File) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Check server response
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bad HTTP status: %s", resp.Status)

	}

	// Writer the body to file
	text := fmt.Sprintf("Downloading %s", url)
	title := fmt.Sprintf("Downloading %s", path.Base(url))
	progressBar := message.NewProgressBar(resp.ContentLength, title)

	if _, err = io.Copy(destinationFile, io.TeeReader(resp.Body, progressBar)); err != nil {
		progressBar.Stop()
		return err
	}

	progressBar.Success(text)
	return nil
}

func sgetFile(url string, destinationFile *os.File, cosignKeyPath string) error {
	// Remove the custom protocol header from the url
	_, url, _ = strings.Cut(url, SGETProtocol)
	return Sget(url, cosignKeyPath, destinationFile, context.TODO())
}
