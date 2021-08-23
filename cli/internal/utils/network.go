package utils

import (
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"

	"github.com/sirupsen/logrus"
)

func IsUrl(source string) bool {
	parsedUrl, err := url.Parse(source)
	return err == nil && parsedUrl.Scheme != "" && parsedUrl.Host != ""
}

func Download(url string) []byte {
	logContext := logrus.WithFields(logrus.Fields{
		"url": url,
	})

	// Get the data
	resp, err := http.Get(url)
	if err != nil {
		logContext.Fatal("Unable to download the file", err)
	}
	defer resp.Body.Close()

	// Check server response
	if resp.StatusCode != http.StatusOK {
		logContext.Fatalf("Bad HTTP status: %s", resp.Status)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		logrus.Warn(err)
		logrus.WithField("Url", url).Fatal("Unable to load the remote text", err)
	}
	return body
}

func DownloadToFile(url string, target string) {

	logContext := logrus.WithFields(logrus.Fields{
		"url":         url,
		"destination": target,
	})

	logContext.Info("Downloading file")

	// Create the file
	destinationFile, err := os.Create(target)
	if err != nil {
		logContext.Fatal("Unable to create the destination file")
	}
	defer destinationFile.Close()

	// Get the data
	resp, err := http.Get(url)
	if err != nil {
		logContext.Fatal("Unable to download the file", err)
	}
	defer resp.Body.Close()

	// Check server response
	if resp.StatusCode != http.StatusOK {
		logContext.Fatalf("Bad HTTP status: %s", resp.Status)
	}

	// Writer the body to file
	_, err = io.Copy(destinationFile, resp.Body)
	if err != nil {
		logContext.Fatal("Unable to save the file", err)
	}
}
