package git

import "regexp"

func transformURL(baseUrl string, url string) (string, string) {
	regex := regexp.MustCompile(`://|[^\w\-\.]`)
	replaced := regex.ReplaceAllString(url, "__")
	return baseUrl + "/bigbang/" + replaced, replaced
}
