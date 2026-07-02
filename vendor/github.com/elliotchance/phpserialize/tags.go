package phpserialize

import "strings"

type tagOptions string

func parseTag(tag string) (string, tagOptions) {
	if i := strings.Index(tag, ","); i != -1 {
		return tag[:i], tagOptions(tag[i+1:])
	}
	return tag, ""
}

func (o tagOptions) Contains(option string) bool {
	if len(o) == 0 {
		return false
	}
	s := string(o)
	for s != "" {
		var next string
		i := strings.Index(s, ",")
		if i >= 0 {
			s, next = s[:i], s[i+1:]
		}
		if s == option {
			return true
		}
		s = next
	}
	return false
}
