Interpolate
===========

[![Build status](https://badge.buildkite.com/3d9081230a965a20d7b68d66564644febdada7f968cc6f9c9c.svg)](https://buildkite.com/buildkite/interpolate)
[![GoDoc](https://godoc.org/github.com/buildkite/interpolate?status.svg)](https://godoc.org/github.com/buildkite/interpolate)

A golang library for parameter expansion (like `${BLAH}` or `$BLAH`) in strings from environment variables. An implementation of [POSIX Parameter Expansion](http://pubs.opengroup.org/onlinepubs/9699919799/utilities/V3_chap02.html#tag_18_06_02), plus some other basic operations that you'd expect in a shell scripting environment [like bash](https://www.gnu.org/software/bash/manual/html_node/Shell-Parameter-Expansion.html).

## Installation

```
go get -u github.com/buildkite/interpolate
```

## Usage

```go
package main

import (
  "github.com/buildkite/interpolate"
  "fmt"
)

func main() {
	env := interpolate.NewSliceEnv([]string{
		"HELLO_WORLD=🦀",
	})

	output, _ := interpolate.Interpolate(env, "Buildkite... ${HELLO_WORLD} ${ANOTHER_VAR:-🏖}")
	fmt.Println(output)
}

// Output: Buildkite... 🦀 🏖

```

## Supported Expansions

<dl>
  <dt><code>${parameter}</code> or <code>$parameter</code></dt>
  <dd><strong>Use value.</strong> If parameter is set, then it shall be substituted; otherwise it will be blank</dd>

  <dt><code>${parameter:-<em>[word]</em>}</code></dt>
  <dd><strong>Use default values.</strong> If parameter is unset or null, the expansion of word (or an empty string if word is omitted) shall be substituted; otherwise, the value of parameter shall be substituted.</dd>

  <dt><code>${parameter-<em>[word]</em>}</code></dt>
  <dd><strong>Use default values when not set.</strong> If parameter is unset, the expansion of word (or an empty string if word is omitted) shall be substituted; otherwise, the value of parameter shall be substituted.</dd>

  <dt><code>${parameter:<em>[offset]</em>}</code></dt>
  <dd><strong>Use the substring of parameter after offset.</strong> A negative offset must be separated from the colon with a space, and will select from the end of the string. If the value is out of bounds, an empty string will be substituted.</dd>

  <dt><code>${parameter:<em>[offset]</em>:<em>[length]</em>}</code></dt>
  <dd><strong>Use the substring of parameter after offset of given length.</strong> A negative offset must be separated from the colon with a space, and will select from the end of the string. If the offset is out of bounds, an empty string will be substituted. If the length is greater than the length then the entire string will be returned.</dd>

  <dt><code>${parameter:?<em>[word]</em>}</code></dt>
  <dd><strong>Indicate Error if Null or Unset.</strong> If parameter is unset or null, the expansion of word (or a message indicating it is unset if word is omitted) shall be returned as an error.</dd>

  <dt><code>$$parameter</code> or <code>\$parameter</code> or <code>$${expression}</code> or <code>\${expression}</code></dt>
  <dd><strong>An escaped interpolation.</strong> Will not be interpolated, but will be unescaped by a call to <code>interpolate.Interpolate()</code></dd>
</dl>

## License

Licensed under MIT license, in `LICENSE`.
