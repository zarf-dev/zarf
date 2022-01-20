package utils

import "regexp"

// For further explanation see https://regex101.com/library/PiL191 and https://regex101.com/r/PiL191/1
var hostParser = regexp.MustCompile(`(?im)^([a-z0-9\-.]+\.[a-z0-9\-]+:?[0-9]*)?/?(.+)$`)

// SwapHost Perform base url replacment without the docker libs
func SwapHost(src string, targetHost string) string {
	var substitution = targetHost + "/$2"
	return hostParser.ReplaceAllString(src, substitution)
}
