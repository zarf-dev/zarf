//go:build stdjson

package json

import (
	"encoding/json"
)

var (
	Marshal    = json.Marshal
	Unmarshal  = json.Unmarshal
	NewDecoder = json.NewDecoder
)
