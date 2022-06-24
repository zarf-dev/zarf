package utils

import (
	"regexp"

	"github.com/defenseunicorns/zarf/src/internal/message"
)

// For further explanation see https://regex101.com/r/DzfGag/1
var hostParser = regexp.MustCompile(`(?im)^(.*):(.*/.*)`)

// SwapHost Perform base url replacment without the docker libs
func SwapHost(src, targetHost string) string {
	message.Debugf("images.SwapHost(%s, %s)", src, targetHost)
	return hostParser.ReplaceAllString(src, targetHost+"/$1.$2")
}
