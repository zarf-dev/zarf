package rpmdb

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"path"
	"strings"
	"time"
)

type PackageInfo struct {
	Epoch           *int
	Name            string
	Version         string
	Release         string
	Arch            string
	SourceRpm       string
	Size            int
	License         string
	Vendor          string
	Modularitylabel string
	Summary         string
	Packager        string
	URL             string
	PGP             string
	SigMD5          string
	RSAHeader       string
	DigestAlgorithm DigestAlgorithm
	InstallTime     int
	BaseNames       []string
	DirIndexes      []int32
	DirNames        []string
	FileSizes       []int32
	FileDigests     []string
	FileModes       []uint16
	FileFlags       []int32
	UserNames       []string
	GroupNames      []string

	Provides []string
	Requires []string
}

type FileInfo struct {
	Path      string
	Mode      uint16
	Digest    string
	Size      int32
	Username  string
	Groupname string
	Flags     FileFlags
}

// ref. https://github.com/rpm-software-management/rpm/blob/rpm-4.14.3-release/lib/tagexts.c#L752
//
//nolint:funlen,gocognit,gocyclo,goconst
func getNEVRA(indexEntries []indexEntry) (*PackageInfo, error) {
	pkgInfo := &PackageInfo{}
	for _, ie := range indexEntries {
		switch ie.Info.Tag {
		case RPMTAG_DIRINDEXES:
			if ie.Info.Type != RPM_INT32_TYPE {
				return nil, errors.New("invalid tag dir indexes")
			}

			dirIndexes, err := parseInt32Array(ie.Data, ie.Length)
			if err != nil {
				return nil, fmt.Errorf("unable to read dir indexes: %w", err)
			}
			pkgInfo.DirIndexes = dirIndexes
		case RPMTAG_DIRNAMES:
			if ie.Info.Type != RPM_STRING_ARRAY_TYPE {
				return nil, errors.New("invalid tag dir names")
			}
			pkgInfo.DirNames = parseStringArray(ie.Data)
		case RPMTAG_BASENAMES:
			if ie.Info.Type != RPM_STRING_ARRAY_TYPE {
				return nil, errors.New("invalid tag base names")
			}
			pkgInfo.BaseNames = parseStringArray(ie.Data)
		case RPMTAG_MODULARITYLABEL:
			if ie.Info.Type != RPM_STRING_TYPE {
				return nil, errors.New("invalid tag modularitylabel")
			}
			pkgInfo.Modularitylabel = string(bytes.TrimRight(ie.Data, "\x00"))
		case RPMTAG_NAME:
			if ie.Info.Type != RPM_STRING_TYPE {
				return nil, errors.New("invalid tag name")
			}
			pkgInfo.Name = string(bytes.TrimRight(ie.Data, "\x00"))
		case RPMTAG_EPOCH:
			if ie.Info.Type != RPM_INT32_TYPE {
				return nil, errors.New("invalid tag epoch")
			}

			if ie.Data != nil {
				value, err := parseInt32(ie.Data)
				if err != nil {
					return nil, fmt.Errorf("failed to parse epoch: %w", err)
				}
				pkgInfo.Epoch = &value
			}
		case RPMTAG_VERSION:
			if ie.Info.Type != RPM_STRING_TYPE {
				return nil, errors.New("invalid tag version")
			}
			pkgInfo.Version = string(bytes.TrimRight(ie.Data, "\x00"))
		case RPMTAG_RELEASE:
			if ie.Info.Type != RPM_STRING_TYPE {
				return nil, errors.New("invalid tag release")
			}
			pkgInfo.Release = string(bytes.TrimRight(ie.Data, "\x00"))
		case RPMTAG_ARCH:
			if ie.Info.Type != RPM_STRING_TYPE {
				return nil, errors.New("invalid tag arch")
			}
			pkgInfo.Arch = string(bytes.TrimRight(ie.Data, "\x00"))
		case RPMTAG_SOURCERPM:
			if ie.Info.Type != RPM_STRING_TYPE {
				return nil, errors.New("invalid tag sourcerpm")
			}
			pkgInfo.SourceRpm = string(bytes.TrimRight(ie.Data, "\x00"))
			if pkgInfo.SourceRpm == "(none)" {
				pkgInfo.SourceRpm = ""
			}
		case RPMTAG_PROVIDENAME:
			if ie.Info.Type != RPM_STRING_ARRAY_TYPE {
				return nil, errors.New("invalid tag providename")
			}
			pkgInfo.Provides = parseStringArray(ie.Data)
		case RPMTAG_REQUIRENAME:
			if ie.Info.Type != RPM_STRING_ARRAY_TYPE {
				return nil, errors.New("invalid tag requirename")
			}
			pkgInfo.Requires = parseStringArray(ie.Data)
		case RPMTAG_LICENSE:
			if ie.Info.Type != RPM_STRING_TYPE {
				return nil, errors.New("invalid tag license")
			}
			pkgInfo.License = string(bytes.TrimRight(ie.Data, "\x00"))
			if pkgInfo.License == "(none)" {
				pkgInfo.License = ""
			}
		case RPMTAG_VENDOR:
			if ie.Info.Type != RPM_STRING_TYPE {
				return nil, errors.New("invalid tag vendor")
			}
			pkgInfo.Vendor = string(bytes.TrimRight(ie.Data, "\x00"))
			if pkgInfo.Vendor == "(none)" {
				pkgInfo.Vendor = ""
			}
		case RPMTAG_PACKAGER:
			if ie.Info.Type != RPM_STRING_TYPE {
				return nil, errors.New("invalid tag packager")
			}
			pkgInfo.Packager = string(bytes.TrimRight(ie.Data, "\x00"))
			if pkgInfo.Packager == "(none)" {
				pkgInfo.Packager = ""
			}
		case RPMTAG_URL:
			if ie.Info.Type != RPM_STRING_TYPE {
				return nil, errors.New("invalid tag url")
			}
			pkgInfo.URL = string(bytes.TrimRight(ie.Data, "\x00"))
			if pkgInfo.URL == "(none)" {
				pkgInfo.URL = ""
			}
		case RPMTAG_SIZE:
			if ie.Info.Type != RPM_INT32_TYPE {
				return nil, errors.New("invalid tag size")
			}

			size, err := parseInt32(ie.Data)
			if err != nil {
				return nil, fmt.Errorf("failed to parse size: %w", err)
			}
			pkgInfo.Size = size
		case RPMTAG_FILEDIGESTALGO:
			// note: all digests within a package entry only supports a single digest algorithm (there may be future support for
			// algorithm noted for each file entry, but currently unimplemented: https://github.com/rpm-software-management/rpm/blob/0b75075a8d006c8f792d33a57eae7da6b66a4591/lib/rpmtag.h#L256)
			if ie.Info.Type != RPM_INT32_TYPE {
				return nil, errors.New("invalid tag digest algo")
			}

			digestAlgorithm, err := parseInt32(ie.Data)
			if err != nil {
				return nil, fmt.Errorf("failed to parse digest algo: %w", err)
			}

			pkgInfo.DigestAlgorithm = DigestAlgorithm(digestAlgorithm)
		case RPMTAG_FILESIZES:
			// note: there is no distinction between int32, uint32, and []uint32
			if ie.Info.Type != RPM_INT32_TYPE {
				return nil, errors.New("invalid tag file-sizes")
			}
			fileSizes, err := parseInt32Array(ie.Data, ie.Length)
			if err != nil {
				return nil, fmt.Errorf("failed to parse file-sizes: %w", err)
			}
			pkgInfo.FileSizes = fileSizes
		case RPMTAG_FILEDIGESTS:
			if ie.Info.Type != RPM_STRING_ARRAY_TYPE {
				return nil, errors.New("invalid tag file-digests")
			}
			pkgInfo.FileDigests = parseStringArray(ie.Data)
		case RPMTAG_FILEMODES:
			// note: there is no distinction between int16, uint16, and []uint16
			if ie.Info.Type != RPM_INT16_TYPE {
				return nil, errors.New("invalid tag file-modes")
			}
			fileModes, err := uint16Array(ie.Data, ie.Length)
			if err != nil {
				return nil, fmt.Errorf("failed to parse file-modes: %w", err)
			}
			pkgInfo.FileModes = fileModes
		case RPMTAG_FILEFLAGS:
			// note: there is no distinction between int32, uint32, and []uint32
			if ie.Info.Type != RPM_INT32_TYPE {
				return nil, errors.New("invalid tag file-flags")
			}
			fileFlags, err := parseInt32Array(ie.Data, ie.Length)
			if err != nil {
				return nil, fmt.Errorf("failed to parse file-flags: %w", err)
			}
			pkgInfo.FileFlags = fileFlags
		case RPMTAG_FILEUSERNAME:
			if ie.Info.Type != RPM_STRING_ARRAY_TYPE {
				return nil, errors.New("invalid tag usernames")
			}
			pkgInfo.UserNames = parseStringArray(ie.Data)
		case RPMTAG_FILEGROUPNAME:
			if ie.Info.Type != RPM_STRING_ARRAY_TYPE {
				return nil, errors.New("invalid tag groupnames")
			}
			pkgInfo.GroupNames = parseStringArray(ie.Data)
		case RPMTAG_SUMMARY:
			// some libraries have a string value instead of international string, so accounting for both
			if ie.Info.Type != RPM_I18NSTRING_TYPE && ie.Info.Type != RPM_STRING_TYPE {
				return nil, errors.New("invalid tag summary")
			}
			// since this is an international string, getting the first null terminated string
			pkgInfo.Summary = string(bytes.Split(ie.Data, []byte{0})[0])
		case RPMTAG_INSTALLTIME:
			if ie.Info.Type != RPM_INT32_TYPE {
				return nil, errors.New("invalid tag installtime")
			}
			installTime, err := parseInt32(ie.Data)
			if err != nil {
				return nil, fmt.Errorf("failed to parse installtime: %w", err)
			}
			pkgInfo.InstallTime = installTime
		case RPMTAG_SIGMD5:
			// It is just string that we need to encode to hex
			digest := ie.Data
			pkgInfo.SigMD5 = hex.EncodeToString(digest)
		case RPMTAG_RSAHEADER:
			if ie.Info.Type != RPM_BIN_TYPE {
				return nil, errors.New("invalid rsa signature")
			}
			val, err := parsePGP(ie)
			if err != nil {
				return nil, fmt.Errorf("failed to parse rsa signature: %w", err)
			}
			pkgInfo.RSAHeader = val
		case RPMTAG_PGP:
			if ie.Info.Type != RPM_BIN_TYPE {
				return nil, errors.New("invalid pgp signature")
			}
			val, err := parsePGP(ie)
			if err != nil {
				return nil, fmt.Errorf("failed to parse pgp signature: %w", err)
			}
			pkgInfo.PGP = val
		}
	}

	return pkgInfo, nil
}

var pubKeyLookup = map[uint8]string{
	0x01: "RSA",
}
var hashLookup = map[uint8]string{
	0x02: "SHA1",
	0x08: "SHA256",
}

// parsePGP parses an OpenPGP signature packet (RFC 4880) stored in an RPM header
// tag (RPMTAG_PGP or RPMTAG_RSAHEADER) and returns a human-readable string
// matching rpm's %{...:pgpsig} format.
func parsePGP(ie indexEntry) (string, error) {
	if len(ie.Data) < 4 {
		return "", nil
	}

	r := bytes.NewReader(ie.Data)

	// Parse PGP packet tag byte (RFC 4880 Section 4.2)
	var tag uint8
	if err := binary.Read(r, binary.BigEndian, &tag); err != nil {
		return "", err
	}
	if tag&0x80 == 0 {
		return "", nil
	}

	// Skip length field to reach the signature body
	skip := func(n int64) error {
		_, err := r.Seek(n, io.SeekCurrent)
		return err
	}
	if tag&0x40 == 0 {
		// Old format (bit 6 = 0): length type in bits 1-0
		var n int64
		switch tag & 0x03 {
		case 0:
			n = 1
		case 1:
			n = 2
		case 2:
			n = 4
		case 3:
			// indeterminate length: body extends to end of input; no skip needed.
		}
		if n > 0 {
			if err := skip(n); err != nil {
				return "", err
			}
		}
	} else {
		// New format (bit 6 = 1)
		var first uint8
		if err := binary.Read(r, binary.BigEndian, &first); err != nil {
			return "", err
		}
		var n int64
		switch {
		case first < 192:
			// 1-byte length, already consumed
		case first < 224:
			n = 1
		case first == 255:
			n = 4
		}
		if n > 0 {
			if err := skip(n); err != nil {
				return "", err
			}
		}
	}

	// Read PGP signature version
	var version uint8
	if err := binary.Read(r, binary.BigEndian, &version); err != nil {
		return "", err
	}

	switch version {
	case 3:
		return parsePGPv3(r)
	case 4:
		return parsePGPv4(r)
	default:
		return "", nil
	}
}

// parsePGPv3 parses a PGP v3 signature body (after the version byte).
// V3 layout: hashMatLen(1) sigType(1) date(4) keyID(8) pubKeyAlgo(1) hashAlgo(1)
func parsePGPv3(r io.Reader) (string, error) {
	var sig struct {
		HashMatLen uint8
		SigType    uint8
		Date       int32
		KeyID      [8]byte
		PubKeyAlgo uint8
		HashAlgo   uint8
	}
	if err := binary.Read(r, binary.BigEndian, &sig); err != nil {
		return "", fmt.Errorf("invalid PGP v3 signature: %w", err)
	}

	pubKey := pubKeyLookup[sig.PubKeyAlgo]
	hash := hashLookup[sig.HashAlgo]
	pkgDate := time.Unix(int64(sig.Date), 0).UTC().Format("Mon Jan _2 15:04:05 2006")

	return fmt.Sprintf("%s/%s, %s, Key ID %x", pubKey, hash, pkgDate, sig.KeyID), nil
}

// parsePGPv4 parses a PGP v4 signature body (after the version byte).
// V4 layout: sigType(1) pubKeyAlgo(1) hashAlgo(1) hashedSubLen(2) hashedSubs(N)
//
//	unhashedSubLen(2) unhashedSubs(N) ...
func parsePGPv4(r io.Reader) (string, error) {
	var header struct {
		SigType    uint8
		PubKeyAlgo uint8
		HashAlgo   uint8
	}
	if err := binary.Read(r, binary.BigEndian, &header); err != nil {
		return "", fmt.Errorf("invalid PGP v4 signature header: %w", err)
	}

	var keyID [8]byte
	var date int64

	// Process hashed and unhashed subpacket areas
	for i := 0; i < 2; i++ {
		var subLen uint16
		if err := binary.Read(r, binary.BigEndian, &subLen); err != nil {
			return "", fmt.Errorf("invalid PGP v4 subpacket length: %w", err)
		}
		subData := make([]byte, subLen)
		if _, err := io.ReadFull(r, subData); err != nil {
			return "", fmt.Errorf("invalid PGP v4 subpacket data: %w", err)
		}
		parsePGPv4Subpackets(subData, &keyID, &date)
	}

	pubKey := pubKeyLookup[header.PubKeyAlgo]
	hash := hashLookup[header.HashAlgo]
	pkgDate := time.Unix(date, 0).UTC().Format("Mon Jan _2 15:04:05 2006")

	return fmt.Sprintf("%s/%s, %s, Key ID %x", pubKey, hash, pkgDate, keyID), nil
}

// PGP v4 subpacket types (RFC 4880 Section 5.2.3.1)
const (
	pgpSubpacketCreationTime      = 2
	pgpSubpacketIssuerKeyID       = 16
	pgpSubpacketIssuerFingerprint = 33
)

// parsePGPv4Subpackets extracts the creation time and key ID from a subpacket area.
// If both an Issuer Key ID (type 16) and Issuer Fingerprint (type 33) subpacket are
// present, the explicit Key ID takes priority.
func parsePGPv4Subpackets(data []byte, keyID *[8]byte, date *int64) {
	var hasExplicitKeyID bool
	offset := 0
	for offset < len(data) {
		// Subpacket length (RFC 4880 Section 5.2.3.1)
		var spLen int
		first := data[offset]
		offset++
		switch {
		case first < 192:
			spLen = int(first)
		case first < 255:
			if offset >= len(data) {
				return
			}
			spLen = (int(first)-192)<<8 + int(data[offset]) + 192
			offset++
		default: // 255
			if offset+4 > len(data) {
				return
			}
			spLen = int(data[offset])<<24 | int(data[offset+1])<<16 | int(data[offset+2])<<8 | int(data[offset+3])
			offset += 4
		}

		if spLen == 0 || offset+spLen > len(data) {
			return
		}

		spType := data[offset] & 0x7f // strip critical bit
		spBody := data[offset+1 : offset+spLen]
		offset += spLen

		switch spType {
		case pgpSubpacketCreationTime:
			if len(spBody) >= 4 {
				*date = int64(spBody[0])<<24 | int64(spBody[1])<<16 | int64(spBody[2])<<8 | int64(spBody[3])
			}
		case pgpSubpacketIssuerKeyID:
			if len(spBody) >= 8 {
				copy(keyID[:], spBody[:8])
				hasExplicitKeyID = true
			}
		case pgpSubpacketIssuerFingerprint:
			// version(1) + fingerprint(20 for v4, 32 for v5); key ID = last 8 bytes.
			// Only use as fallback when no explicit Issuer Key ID subpacket is present.
			if !hasExplicitKeyID && len(spBody) >= 9 {
				copy(keyID[:], spBody[len(spBody)-8:])
			}
		}
	}
}

const (
	sizeOfInt32  = 4
	sizeOfUInt16 = 2
)

func parseInt32Array(data []byte, arraySize int) ([]int32, error) {
	length := arraySize / sizeOfInt32
	values := make([]int32, length)
	reader := bytes.NewReader(data)
	if err := binary.Read(reader, binary.BigEndian, &values); err != nil {
		return nil, fmt.Errorf("failed to read binary: %w", err)
	}
	return values, nil
}

func parseInt32(data []byte) (int, error) {
	var value int32
	reader := bytes.NewReader(data)
	if err := binary.Read(reader, binary.BigEndian, &value); err != nil {
		return 0, fmt.Errorf("failed to read binary: %w", err)
	}
	return int(value), nil
}

func uint16Array(data []byte, arraySize int) ([]uint16, error) {
	length := arraySize / sizeOfUInt16
	values := make([]uint16, length)
	reader := bytes.NewReader(data)
	if err := binary.Read(reader, binary.BigEndian, &values); err != nil {
		return nil, fmt.Errorf("failed to read binary: %w", err)
	}
	return values, nil
}

func parseStringArray(data []byte) []string {
	return strings.Split(string(bytes.TrimRight(data, "\x00")), "\x00")
}

func (p *PackageInfo) InstalledFileNames() ([]string, error) {
	if p == nil || len(p.DirNames) == 0 || len(p.DirIndexes) == 0 || len(p.BaseNames) == 0 {
		return nil, nil
	}

	// ref. https://github.com/rpm-software-management/rpm/blob/rpm-4.14.3-release/lib/tagexts.c#L68-L70
	if len(p.DirIndexes) != len(p.BaseNames) || len(p.DirNames) > len(p.BaseNames) {
		return nil, fmt.Errorf("invalid rpm %s", p.Name)
	}

	var filePaths []string
	for i, baseName := range p.BaseNames {
		idx := p.DirIndexes[i]
		if len(p.DirNames) <= int(idx) {
			return nil, fmt.Errorf("invalid rpm %s", p.Name)
		}
		dir := p.DirNames[idx]
		filePaths = append(filePaths, path.Join(dir, baseName)) // should be slash-separated
	}
	return filePaths, nil
}

func (p *PackageInfo) InstalledFiles() ([]FileInfo, error) {
	fileNames, err := p.InstalledFileNames()
	if err != nil {
		return nil, err
	}

	var files []FileInfo
	for i, fileName := range fileNames {
		var digest, username, groupname string
		var mode uint16
		var size, flags int32

		if p.FileDigests != nil && len(p.FileDigests) > i {
			digest = p.FileDigests[i]
		}

		if p.FileModes != nil && len(p.FileModes) > i {
			mode = p.FileModes[i]
		}

		if p.FileSizes != nil && len(p.FileSizes) > i {
			size = p.FileSizes[i]
		}

		if p.UserNames != nil && len(p.UserNames) > i {
			username = p.UserNames[i]
		}

		if p.GroupNames != nil && len(p.GroupNames) > i {
			groupname = p.GroupNames[i]
		}

		if p.FileFlags != nil && len(p.FileFlags) > i {
			flags = p.FileFlags[i]
		}

		record := FileInfo{
			Path:      fileName,
			Mode:      mode,
			Digest:    digest,
			Size:      size,
			Username:  username,
			Groupname: groupname,
			Flags:     FileFlags(flags),
		}
		files = append(files, record)
	}

	return files, nil
}

func (p *PackageInfo) EpochNum() int {
	if p.Epoch == nil {
		return 0
	}
	return *p.Epoch
}
