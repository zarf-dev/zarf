package rustaudit

import (
	"bytes"
	"compress/zlib"
	"debug/elf"
	"debug/macho"
	"debug/pe"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

// This struct is embedded in dependencies produced with rust-audit:
// https://github.com/Shnatsel/rust-audit/blob/bc805a8fdd1492494179bd01a598a26ec22d44fe/auditable-serde/src/lib.rs#L89
type VersionInfo struct {
	Packages []Package `json:"packages"`
}

type DependencyKind string

const (
	Build   DependencyKind = "build"
	Runtime DependencyKind = "runtime"
)

type Package struct {
	Name         string         `json:"name"`
	Version      string         `json:"version"`
	Source       string         `json:"source"`
	Kind         DependencyKind `json:"kind"`
	Dependencies []uint         `json:"dependencies"`
	Features     []string       `json:"features"` // Removed in cargo-auditable 0.5.0
	Root         bool           `json:"root"`
}

// Default the Kind to Runtime during unmarshalling
func (p *Package) UnmarshalJSON(text []byte) error {
	type pkgty Package
	pkg := pkgty{
		Kind: Runtime,
	}
	if err := json.Unmarshal(text, &pkg); err != nil {
		return err
	}
	*p = Package(pkg)
	return nil
}

var (
	// Returned if an executable is not a known format
	ErrUnknownFileFormat = errors.New("unknown file format")
	// errNoRustDepInfo is returned when an executable file doesn't contain Rust dependency information
	ErrNoRustDepInfo = errors.New("rust dependency information not found")

	// Headers for different binary types
	elfHeader               = []byte("\x7FELF")
	peHeader                = []byte("MZ")
	machoHeader             = []byte("\xFE\xED\xFA")
	machoHeaderLittleEndian = []byte("\xFA\xED\xFE")
	machoUniversalHeader    = []byte("\xCA\xFE\xBA\xBE")
	// https://www.w3.org/TR/2019/REC-wasm-core-1-20191205/#binary-magic
	wasmHeader = []byte("\x00asm\x01\x00\x00\x00")

	cargoAuditableSectionName       = ".dep-v0"
	cargoAuditableLegacySectionName = ".rust-deps-v0"
)

func GetDependencyInfo(r io.ReaderAt) (VersionInfo, error) {
	// Read file header
	header := make([]byte, 16)
	n, err := r.ReadAt(header, 0)
	if n < len(header) || err != nil {
		return VersionInfo{}, ErrUnknownFileFormat
	}

	var x exe
	switch {
	case bytes.HasPrefix(header, elfHeader):
		f, err := elf.NewFile(r)
		if err != nil {
			return VersionInfo{}, ErrUnknownFileFormat
		}
		x = &elfExe{f}
	case bytes.HasPrefix(header, peHeader):
		f, err := pe.NewFile(r)
		if err != nil {
			return VersionInfo{}, ErrUnknownFileFormat
		}
		x = &peExe{f}
	case bytes.HasPrefix(header, machoHeader) || bytes.HasPrefix(header[1:], machoHeaderLittleEndian) || bytes.HasPrefix(header, machoUniversalHeader):
		f, err := macho.NewFile(r)
		if err != nil {
			return VersionInfo{}, ErrUnknownFileFormat
		}
		x = &machoExe{f}
	case bytes.HasPrefix(header, wasmHeader):
		x = &wasmReader{r}
	default:
		return VersionInfo{}, ErrUnknownFileFormat
	}

	data, err := x.ReadRustDepSection()
	if err != nil {
		return VersionInfo{}, err
	}

	// The json is compressed using zlib, so decompress it
	b := bytes.NewReader(data)
	reader, err := zlib.NewReader(b)

	if err != nil {
		return VersionInfo{}, fmt.Errorf("section not compressed: %w", err)
	}

	buf, err := io.ReadAll(reader)
	reader.Close()

	if err != nil {
		return VersionInfo{}, fmt.Errorf("failed to decompress JSON: %w", err)
	}

	var versionInfo VersionInfo
	err = json.Unmarshal(buf, &versionInfo)
	if err != nil {
		return VersionInfo{}, fmt.Errorf("failed to unmarshall JSON: %w", err)
	}

	return versionInfo, nil
}

// Interface for binaries that may have a Rust dependencies section
type exe interface {
	ReadRustDepSection() ([]byte, error)
}

type elfExe struct {
	f *elf.File
}

func (x *elfExe) ReadRustDepSection() ([]byte, error) {
	// Try .dep-v0 first, falling back to .rust-deps-v0 as used in
	// in rust-audit 0.1.0
	depInfo := x.f.Section(cargoAuditableSectionName)

	if depInfo != nil {
		return depInfo.Data()
	}

	depInfo = x.f.Section(cargoAuditableLegacySectionName)

	if depInfo == nil {
		return nil, ErrNoRustDepInfo
	}

	return depInfo.Data()
}

type peExe struct {
	f *pe.File
}

func (x *peExe) ReadRustDepSection() ([]byte, error) {
	// Try .dep-v0 first, falling back to rdep-v0 as used in
	// in rust-audit 0.1.0
	depInfo := x.f.Section(cargoAuditableSectionName)

	if depInfo != nil {
		return depInfo.Data()
	}

	depInfo = x.f.Section("rdep-v0")

	if depInfo == nil {
		return nil, ErrNoRustDepInfo
	}

	return depInfo.Data()
}

type machoExe struct {
	f *macho.File
}

func (x *machoExe) ReadRustDepSection() ([]byte, error) {
	// Try .dep-v0 first, falling back to rust-deps-v0 as used in
	// in rust-audit 0.1.0
	depInfo := x.f.Section(cargoAuditableSectionName)

	if depInfo != nil {
		return depInfo.Data()
	}

	depInfo = x.f.Section("rust-deps-v0")

	if depInfo == nil {
		return nil, ErrNoRustDepInfo
	}

	return depInfo.Data()
}

type wasmReader struct {
	r io.ReaderAt
}

func (x *wasmReader) ReadRustDepSection() ([]byte, error) {
	r := x.r
	var offset int64 = 0

	// Check the preamble (magic number and version)
	buf := make([]byte, 8)
	_, err := r.ReadAt(buf, offset)
	offset += 8
	if err != nil || !bytes.Equal(buf, wasmHeader) {
		return nil, ErrUnknownFileFormat
	}

	// https://www.w3.org/TR/2019/REC-wasm-core-1-20191205/#custom-section%E2%91%A0
	// Look through the sections until we find a custom .dep-v0 section or EOF
	for {
		// Read single byte section ID
		sectionId := make([]byte, 1)
		_, err = r.ReadAt(sectionId, offset)
		offset += 1
		if err == io.EOF {
			return nil, ErrNoRustDepInfo
		} else if err != nil {
			return nil, ErrUnknownFileFormat
		}

		// Read section size
		buf = make([]byte, 4)
		_, err = r.ReadAt(buf, offset)
		if err != nil {
			return nil, ErrUnknownFileFormat
		}
		sectionSize, n, err := readUint32(buf)
		if err != nil {
			return nil, ErrUnknownFileFormat
		}
		offset += n
		nextSection := offset + int64(sectionSize)

		// Custom sections have a zero section ID
		if sectionId[0] != 0 {
			offset = nextSection
			continue
		}

		// The custom section has a variable length name
		// followed by the data
		_, err = r.ReadAt(buf, offset)
		if err != nil {
			return nil, ErrUnknownFileFormat
		}
		nameSize, n, err := readUint32(buf)
		if err != nil {
			return nil, ErrUnknownFileFormat
		}
		offset += n

		// Read section name
		name := make([]byte, nameSize)
		_, err = r.ReadAt(name, offset)
		if err != nil {
			return nil, ErrUnknownFileFormat
		}
		offset += int64(nameSize)

		// Is this our custom section?
		if string(name) != cargoAuditableSectionName {
			offset = nextSection
			continue
		}

		// Read audit data
		data := make([]byte, nextSection-offset)
		_, err = r.ReadAt(data, offset)
		if err != nil {
			return nil, ErrUnknownFileFormat
		}
		return data, nil

	}
}

// wrap binary.Uvarint to return uint32, checking for overflow
// https://www.w3.org/TR/2019/REC-wasm-core-1-20191205/#integers%E2%91%A4
func readUint32(buf []byte) (uint32, int64, error) {
	v, n := binary.Uvarint(buf)
	if n <= 0 || v > uint64(^uint32(0)) {
		return 0, 0, fmt.Errorf("overflow decoding uint32")
	}
	return uint32(v), int64(n), nil
}
