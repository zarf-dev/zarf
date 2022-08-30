package utils

import "regexp"

// For further explanation see https://regex101.com/library/PiL191 and https://regex101.com/r/xVUd5j/1
var hostParser = regexp.MustCompile(`(?im)^((?:(?:https|http)://)?[\w\-.]+[\.|\-|:][\w\-]+)?/?(.+)$`)

// SwapHost Perform base url replacment without the docker libs
func SwapHost(src string, targetHost string) string {
	var substitution = targetHost + "/$2"
	return hostParser.ReplaceAllString(src, substitution)
}
