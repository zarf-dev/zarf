package image

import "github.com/google/go-containerregistry/pkg/name"

func ParseReference(imageStr string) (imageRef string, originalRef string, err error) {
	ref, err := name.ParseReference(imageStr, name.WithDefaultRegistry(""))
	if err != nil {
		return "", "", err
	}
	tag, ok := ref.(name.Tag)
	if ok {
		imageStr = tag.Name()
		originalRef = tag.String() // blindly takes the original input passed into Tag
	}
	return imageStr, originalRef, nil
}
