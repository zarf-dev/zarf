package gguf_parser

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/gpustack/gguf-parser-go/util/funcx"
	"github.com/gpustack/gguf-parser-go/util/ptr"
)

// GGUFFilename represents a GGUF filename,
// see https://github.com/ggerganov/ggml/blob/master/docs/gguf.md#gguf-naming-convention.
type GGUFFilename struct {
	BaseName   string `json:"baseName"`
	SizeLabel  string `json:"sizeLabel"`
	FineTune   string `json:"fineTune"`
	Version    string `json:"version"`
	Encoding   string `json:"encoding"`
	Type       string `json:"type"`
	Shard      *int   `json:"shard,omitempty"`
	ShardTotal *int   `json:"shardTotal,omitempty"`
}

var GGUFFilenameRegex = regexp.MustCompile(`^(?P<BaseName>[A-Za-z\s][A-Za-z0-9._\s]*(?:(?:-(?:(?:[A-Za-z\s][A-Za-z0-9._\s]*)|(?:[0-9._\s]*)))*))-(?:(?P<SizeLabel>(?:\d+x)?(?:\d+\.)?\d+[A-Za-z](?:-[A-Za-z]+(\d+\.)?\d+[A-Za-z]+)?)(?:-(?P<FineTune>[A-Za-z][A-Za-z0-9\s_-]+[A-Za-z](?i:[^BFKIQ])))?)?(?:-(?P<Version>[vV]\d+(?:\.\d+)*))?(?i:-(?P<Encoding>(BF16|F32|F16|([KI]?Q[0-9][A-Z0-9_]*))))?(?:-(?P<Type>LoRA|vocab))?(?:-(?P<Shard>\d{5})-of-(?P<ShardTotal>\d{5}))?\.gguf$`) // nolint:lll

// ParseGGUFFilename parses the given GGUF filename string,
// and returns the GGUFFilename, or nil if the filename is invalid.
func ParseGGUFFilename(name string) *GGUFFilename {
	n := name
	if !strings.HasSuffix(n, ".gguf") {
		n += ".gguf"
	}

	m := make(map[string]string)
	{
		r := GGUFFilenameRegex.FindStringSubmatch(n)
		for i, ne := range GGUFFilenameRegex.SubexpNames() {
			if i != 0 && i <= len(r) {
				m[ne] = r[i]
			}
		}
	}
	if m["BaseName"] == "" {
		return nil
	}

	var gn GGUFFilename
	gn.BaseName = strings.ReplaceAll(m["BaseName"], "-", " ")
	gn.SizeLabel = m["SizeLabel"]
	gn.FineTune = m["FineTune"]
	gn.Version = m["Version"]
	gn.Encoding = m["Encoding"]
	gn.Type = m["Type"]
	if v := m["Shard"]; v != "" {
		gn.Shard = ptr.To(parseInt(v))
	}
	if v := m["ShardTotal"]; v != "" {
		gn.ShardTotal = ptr.To(parseInt(v))
	}
	return &gn
}

func (gn GGUFFilename) String() string {
	if gn.BaseName == "" {
		return ""
	}

	var sb strings.Builder
	sb.WriteString(strings.ReplaceAll(gn.BaseName, " ", "-"))
	if gn.SizeLabel != "" {
		sb.WriteString("-")
		sb.WriteString(gn.SizeLabel)
	}
	if gn.FineTune != "" {
		sb.WriteString("-")
		sb.WriteString(gn.FineTune)
	}
	if gn.Version != "" {
		sb.WriteString("-")
		sb.WriteString(gn.Version)
	}
	if gn.Encoding != "" {
		sb.WriteString("-")
		sb.WriteString(gn.Encoding)
	}
	if gn.Type != "" {
		sb.WriteString("-")
		sb.WriteString(gn.Type)
	}
	if m, n := ptr.Deref(gn.Shard, 0), ptr.Deref(gn.ShardTotal, 0); m > 0 && n > 0 {
		sb.WriteString("-")
		sb.WriteString(fmt.Sprintf("%05d", m))
		sb.WriteString("-of-")
		sb.WriteString(fmt.Sprintf("%05d", n))
	}
	sb.WriteString(".gguf")
	return sb.String()
}

// IsShard returns true if the GGUF filename is a shard.
func (gn GGUFFilename) IsShard() bool {
	return ptr.Deref(gn.Shard, 0) > 0 && ptr.Deref(gn.ShardTotal, 0) > 0
}

var ShardGGUFFilenameRegex = regexp.MustCompile(`^(?P<Prefix>.*)-(?:(?P<Shard>\d{5})-of-(?P<ShardTotal>\d{5}))\.gguf$`)

// IsShardGGUFFilename returns true if the given filename is a shard GGUF filename.
func IsShardGGUFFilename(name string) bool {
	n := name
	if !strings.HasSuffix(n, ".gguf") {
		n += ".gguf"
	}

	m := make(map[string]string)
	{
		r := ShardGGUFFilenameRegex.FindStringSubmatch(n)
		for i, ne := range ShardGGUFFilenameRegex.SubexpNames() {
			if i != 0 && i <= len(r) {
				m[ne] = r[i]
			}
		}
	}

	var shard, shardTotal int
	if v := m["Shard"]; v != "" {
		shard = parseInt(v)
	}
	if v := m["ShardTotal"]; v != "" {
		shardTotal = parseInt(v)
	}
	return shard > 0 && shardTotal > 0
}

// CompleteShardGGUFFilename returns the list of shard GGUF filenames that are related to the given shard GGUF filename.
//
// Only available if the given filename is a shard GGUF filename.
func CompleteShardGGUFFilename(name string) []string {
	n := name
	if !strings.HasSuffix(n, ".gguf") {
		n += ".gguf"
	}

	m := make(map[string]string)
	{
		r := ShardGGUFFilenameRegex.FindStringSubmatch(n)
		for i, ne := range ShardGGUFFilenameRegex.SubexpNames() {
			if i != 0 && i <= len(r) {
				m[ne] = r[i]
			}
		}
	}

	var shard, shardTotal int
	if v := m["Shard"]; v != "" {
		shard = parseInt(v)
	}
	if v := m["ShardTotal"]; v != "" {
		shardTotal = parseInt(v)
	}

	if shard <= 0 || shardTotal <= 0 {
		return nil
	}

	names := make([]string, 0, shardTotal)
	for i := 1; i <= shardTotal; i++ {
		names = append(names, fmt.Sprintf("%s-%05d-of-%05d.gguf", m["Prefix"], i, shardTotal))
	}
	return names
}

func parseInt(v string) int {
	return int(funcx.MustNoError(strconv.ParseInt(v, 10, 64)))
}
