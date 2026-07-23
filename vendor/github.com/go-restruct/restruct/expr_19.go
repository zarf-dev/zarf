// +build go1.9

package restruct

import (
	"math/bits"

	"github.com/go-restruct/restruct/expr"
)

var exprStdLib = map[string]expr.Value{
	"bits": expr.ValueOf(expr.NewPackage(map[string]expr.Value{
		"LeadingZeros":   expr.ValueOf(bits.LeadingZeros),
		"LeadingZeros8":  expr.ValueOf(bits.LeadingZeros8),
		"LeadingZeros16": expr.ValueOf(bits.LeadingZeros16),
		"LeadingZeros32": expr.ValueOf(bits.LeadingZeros32),
		"LeadingZeros64": expr.ValueOf(bits.LeadingZeros64),

		"Len":   expr.ValueOf(bits.Len),
		"Len8":  expr.ValueOf(bits.Len8),
		"Len16": expr.ValueOf(bits.Len16),
		"Len32": expr.ValueOf(bits.Len32),
		"Len64": expr.ValueOf(bits.Len64),

		"OnesCount":   expr.ValueOf(bits.OnesCount),
		"OnesCount8":  expr.ValueOf(bits.OnesCount8),
		"OnesCount16": expr.ValueOf(bits.OnesCount16),
		"OnesCount32": expr.ValueOf(bits.OnesCount32),
		"OnesCount64": expr.ValueOf(bits.OnesCount64),

		"Reverse":   expr.ValueOf(bits.Reverse),
		"Reverse8":  expr.ValueOf(bits.Reverse8),
		"Reverse16": expr.ValueOf(bits.Reverse16),
		"Reverse32": expr.ValueOf(bits.Reverse32),
		"Reverse64": expr.ValueOf(bits.Reverse64),

		"ReverseBytes":   expr.ValueOf(bits.ReverseBytes),
		"ReverseBytes16": expr.ValueOf(bits.ReverseBytes16),
		"ReverseBytes32": expr.ValueOf(bits.ReverseBytes32),
		"ReverseBytes64": expr.ValueOf(bits.ReverseBytes64),

		"RotateLeft":   expr.ValueOf(bits.RotateLeft),
		"RotateLeft8":  expr.ValueOf(bits.RotateLeft8),
		"RotateLeft16": expr.ValueOf(bits.RotateLeft16),
		"RotateLeft32": expr.ValueOf(bits.RotateLeft32),
		"RotateLeft64": expr.ValueOf(bits.RotateLeft64),

		"TrailingZeros":   expr.ValueOf(bits.TrailingZeros),
		"TrailingZeros8":  expr.ValueOf(bits.TrailingZeros8),
		"TrailingZeros16": expr.ValueOf(bits.TrailingZeros16),
		"TrailingZeros32": expr.ValueOf(bits.TrailingZeros32),
		"TrailingZeros64": expr.ValueOf(bits.TrailingZeros64),
	})),
}
