# indent ![build status](https://travis-ci.org/pborman/indent.svg?branch=master)[![GoDoc](https://godoc.org/github.com/pborman/indent?status.svg)](http://godoc.org/github.com/pborman/indent)

The indent package indents lines of text with a prefix.  It supports indent
blocks of text (string or []byte) as well as providing an io.Writer interface.
It is a drop-in replacement for the github.com/openconfig/goyang/pkg/indent package.

For example, indenting the text in the first block below with a ```>>``` prefix
using any of the methods will result in the second block below.
```
Line 1
Line 2
Line 3
```
```
>>Line 1
>>Line 2
>>Line 3
```

The New function is used to create a writer.  Writers may be nested.  For example the
following code produces the output that follows in the next block.
```
package main

import (
	"fmt"
	"os"

	"github.com/pborman/indent"
)

func main() {
	w := indent.New(os.Stdout, "// ")

	fmt.Fprintf(w, "type foo struct {\n")
	wi := indent.New(w, "\t")
	fmt.Fprintf(wi, "A string\n")
	fmt.Fprintf(wi, "B int\n")
	fmt.Fprintf(w, "}\n")
}
```

```
// type foo struct {
// 	A string
// 	B int
// }
```

The New function is intelligent about nesting so the written text is only
processed once prior to sending to the original io.Writer.
