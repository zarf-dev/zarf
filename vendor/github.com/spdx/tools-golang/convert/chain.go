// SPDX-License-Identifier: Apache-2.0 OR GPL-2.0-or-later

package convert

import (
	"fmt"
	"reflect"

	converter "github.com/anchore/go-struct-converter"

	"github.com/spdx/tools-golang/spdx/common"
	"github.com/spdx/tools-golang/spdx/v2/v2_2"
	"github.com/spdx/tools-golang/spdx/v2/v2_3"
	"github.com/spdx/tools-golang/spdx/v3/v3_0"
)

func DocumentChain() converter.FuncChain {
	return converter.NewFuncChain(
		v2_2.From_v2_1, v2_2.To_v2_1,
		v2_3.From_v2_2, v2_3.To_v2_2,
		v3_0.From_v2_3,
		// for future v3.x to v3.y conversions, see funcChain.AutoPackageConverter()
	)
}

// Document converts from one document to another document
// For example, converting a document to the latest version could be done like:
//
// sourceDoc := // e.g. a v2_2.Document from somewhere
// var targetDoc spdx.Document // this can be any document version
// err := convert.Document(sourceDoc, &targetDoc) // the target must be passed as a pointer
func Document(from common.AnyDocument, to common.AnyDocument) error {
	if !IsPtr(to) {
		return fmt.Errorf("struct to convert to must be a pointer")
	}
	from = FromPtr(from)
	if reflect.TypeOf(from) == reflect.TypeOf(FromPtr(to)) {
		reflect.ValueOf(to).Elem().Set(reflect.ValueOf(from))
		return nil
	}
	return DocumentChain().Convert(from, to)
}
