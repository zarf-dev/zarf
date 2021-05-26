package git

import "regexp"

func transformURL(baseUrl string, url string) (string, string) {
	matchRegex := regexp.MustCompile(`^https?:\/\/(?P<user>[^:]+)`)
	replaceRegex := regexp.MustCompile(`^[^@]+@|^https:\/\/|[^\w\-\.]+`)

	matches := matchRegex.FindStringSubmatch(baseUrl)
	account := matches[matchRegex.SubexpIndex("user")]

	replaced := "mirror" + replaceRegex.ReplaceAllString(url, "__")

	return baseUrl + "/" + account + "/" + replaced, replaced
}
