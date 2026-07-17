package devslog

import (
	"log/slog"
)

type attributes []slog.Attr

func (a attributes) Len() int      { return len(a) }
func (a attributes) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a attributes) Less(i, j int) bool {
	if a[i].Value.Kind() == slog.KindGroup && a[j].Value.Kind() != slog.KindGroup {
		return false
	} else if a[i].Value.Kind() != slog.KindGroup && a[j].Value.Kind() == slog.KindGroup {
		return true
	}

	return a[i].Key < a[j].Key
}
