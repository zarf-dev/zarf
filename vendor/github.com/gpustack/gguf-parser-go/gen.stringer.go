//go:build stringer

//go:generate go run golang.org/x/tools/cmd/stringer -linecomment -type GGUFMagic -output zz_generated.ggufmagic.stringer.go -trimprefix GGUFMagic
//go:generate go run golang.org/x/tools/cmd/stringer -linecomment -type GGUFVersion -output zz_generated.ggufversion.stringer.go -trimprefix GGUFVersion
//go:generate go run golang.org/x/tools/cmd/stringer -linecomment -type GGUFMetadataValueType -output zz_generated.ggufmetadatavaluetype.stringer.go -trimprefix GGUFMetadataValueType
//go:generate go run golang.org/x/tools/cmd/stringer -linecomment -type GGUFFileType -output zz_generated.gguffiletype.stringer.go -trimprefix GGUFFileType
//go:generate go run golang.org/x/tools/cmd/stringer -linecomment -type GGMLType -output zz_generated.ggmltype.stringer.go -trimprefix GGMLType
package gguf_parser

import _ "golang.org/x/tools/cmd/stringer"
