package utils

import (
	"fmt"
	"hash/crc32"

	"github.com/distribution/distribution/v3/reference"
)

type Image struct {
	Host      string
	Name      string
	Path      string
	Tag       string
	Digest    string
	Reference string
}

// SwapHost Perform base url replacement and adds a crc32 of the original url to the end of the src
func SwapHost(src string, targetHost string) (string, error) {
	targetImage, err := getTargetImageFromURL(src)
	return targetHost + "/" + targetImage, err
}

// SwapHostWithoutChecksum Perform base url replacement but avoids adding a checksum of the original url.
func SwapHostWithoutChecksum(src string, targetHost string) (string, error) {
	image, err := parseImageURL(src)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s/%s", targetHost, image.Path), nil
}

func getTargetImageFromURL(src string) (string, error) {
	image, err := parseImageURL(src)
	if err != nil {
		return "", err
	}

	// Generate a crc32 hash of the image host + name
	table := crc32.MakeTable(crc32.IEEE)
	checksum := crc32.Checksum([]byte(image.Name), table)

	return fmt.Sprintf("%s-%d", image.Path, checksum), nil
}

func parseImageURL(src string) (out Image, err error) {
	ref, err := reference.Parse(src)
	if err != nil {
		return out, err
	}

	if named, ok := ref.(reference.Named); ok {
		out.Name = named.Name()
		out.Path = reference.Path(named)
		out.Host = reference.Domain(named)
		out.Reference = ref.String()
	} else {
		return out, fmt.Errorf("unable to parse image name from %s", src)
	}

	if tagged, ok := ref.(reference.Tagged); ok {
		out.Tag = tagged.Tag()
	}

	if digested, ok := ref.(reference.Digested); ok {
		out.Digest = digested.Digest().String()
	}

	return out, nil
}
