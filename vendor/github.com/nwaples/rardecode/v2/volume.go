package rardecode

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

var (
	ErrVerMismatch      = errors.New("rardecode: volume version mistmatch")
	ErrArchiveNameEmpty = errors.New("rardecode: archive name empty")
	ErrFileNameRequired = errors.New("rardecode: filename required for multi volume archive")
	ErrInvalidHeaderOff = errors.New("rardecode: invalid filed header offset")

	defaultFS = osFS{}
)

const (
	DefaultMaxDictionarySize = 4 << 30 // default max dictionary size of 4GB
)

type osFS struct{}

func (fs osFS) Open(name string) (fs.File, error) {
	return os.Open(name)
}

type options struct {
	bsize       int     // size to be use for bufio.Reader
	maxDictSize int64   // max dictionary size
	fs          fs.FS   // filesystem to use to open files
	pass        *string // password for encrypted volumes
	skipCheck   bool
	openCheck   bool
}

// An Option is used for optional archive extraction settings.
type Option func(*options)

// BufferSize sets the size of the bufio.Reader used in reading the archive.
func BufferSize(size int) Option {
	return func(o *options) { o.bsize = size }
}

// MaxDictionarySize sets the maximum size in bytes of the dictionary used in decoding a file.
// Any attempt to decode a file with a larger size will return an error.
// The default size if not set is DefaultMaxDictionarySize.
// Any size above 64GB will be ignored. Any size below 256kB will prevent any file from being decoded.
func MaxDictionarySize(size int64) Option {
	return func(o *options) { o.maxDictSize = size }
}

// FileSystem sets the fs.FS to be used for opening archive volumes.
func FileSystem(fs fs.FS) Option {
	return func(o *options) { o.fs = fs }
}

// Password sets the password to use for decrypting archives.
func Password(pass string) Option {
	return func(o *options) { o.pass = &pass }
}

// SkipCheck sets archive files checksum not to be checked.
func SkipCheck(o *options) { o.skipCheck = true }

// OpenFSCheck flags the archive files to be checked on Open or List.
func OpenFSCheck(o *options) { o.openCheck = true }

func getOptions(opts []Option) *options {
	opt := &options{
		fs:          defaultFS,
		maxDictSize: DefaultMaxDictionarySize,
	}
	for _, f := range opts {
		f(opt)
	}
	// truncate password
	if opt.pass != nil {
		runes := []rune(*opt.pass)
		if len(runes) > maxPassword {
			pw := string(runes[:maxPassword])
			opt.pass = &pw
		}
	}
	return opt
}

type volume interface {
	byteReader
	nextBlock() (*fileBlockHeader, error)
	openBlock(volnum int, offset, size int64) error
	canSeek() bool
}

type readerVolume struct {
	br  *bufVolumeReader // buffered reader for current volume file
	n   int64            // bytes left in current block
	num int              // current volume number
	ver int              // archive file format version
	arc archiveBlockReader
	opt *options
}

func (v *readerVolume) init(r io.Reader, volnum int) error {
	var err error
	if v.br == nil {
		v.br, err = newBufVolumeReader(r, v.opt.bsize)
	} else {
		err = v.br.Reset(r)
	}
	if err != nil {
		return err
	}
	if v.arc == nil {
		switch v.br.ver {
		case archiveVersion15:
			v.arc = newArchive15(v.opt.pass)
		case archiveVersion50:
			v.arc = newArchive50(v.opt.pass)
		default:
			return ErrUnknownVersion
		}
		v.ver = v.br.ver
	} else if v.ver != v.br.ver {
		return ErrVerMismatch
	}
	n, err := v.arc.init(v.br)
	if err != nil {
		return err
	}
	v.num = volnum
	if n >= 0 && n != volnum {
		return ErrBadVolumeNumber
	}
	return nil
}

func (v *readerVolume) nextBlock() (*fileBlockHeader, error) {
	if v.n > 0 {
		err := v.br.Discard(v.n)
		if err != nil {
			return nil, err
		}
		v.n = 0
	}
	f, err := v.arc.nextBlock(v.br)
	if err != nil {
		return nil, err
	}
	f.volnum = v.num
	f.dataOff = v.br.off
	v.n = f.PackedSize
	return f, nil
}

func (v *readerVolume) Read(p []byte) (int, error) {
	if v.n == 0 {
		return 0, io.EOF
	}
	if v.n < int64(len(p)) {
		p = p[:v.n]
	}
	n, err := v.br.Read(p)
	v.n -= int64(n)
	if err == io.EOF && v.n > 0 {
		err = io.ErrUnexpectedEOF
	}
	return n, err
}

func (v *readerVolume) ReadByte() (byte, error) {
	if v.n == 0 {
		return 0, io.EOF
	}
	b, err := v.br.ReadByte()
	if err == nil {
		v.n--
	} else if err == io.EOF && v.n > 0 {
		err = io.ErrUnexpectedEOF
	}
	return b, err
}

func (v *readerVolume) canSeek() bool {
	return v.br.canSeek()
}

func (v *readerVolume) openBlock(volnum int, offset, size int64) error {
	if v.num != volnum {
		return ErrBadVolumeNumber
	}
	err := v.br.seek(offset)
	if err != nil {
		return err
	}
	v.n = size
	return nil
}

func newVolume(r io.Reader, opt *options, volnum int) (*readerVolume, error) {
	v := &readerVolume{opt: opt}
	err := v.init(r, volnum)
	if err != nil {
		return nil, err
	}
	return v, nil
}

type fileVolume struct {
	*readerVolume
	f  fs.File
	vm *volumeManager
}

func (v *fileVolume) Close() error { return v.f.Close() }

func (v *fileVolume) open(volnum int) error {
	err := v.Close()
	if err != nil {
		return err
	}
	f, err := v.vm.openVolumeFile(volnum)
	if err != nil {
		return err
	}
	err = v.readerVolume.init(f, volnum)
	if err != nil {
		f.Close()
		return err
	}
	v.f = f
	return nil
}

func (v *fileVolume) openBlock(volnum int, offset, size int64) error {
	if v.num != volnum {
		err := v.open(volnum)
		if err != nil {
			return err
		}
	}
	return v.readerVolume.openBlock(volnum, offset, size)
}

func (v *fileVolume) openNext() error { return v.open(v.num + 1) }

func (v *fileVolume) nextBlock() (*fileBlockHeader, error) {
	for {
		h, err := v.readerVolume.nextBlock()
		if err == nil {
			return h, nil
		}
		if err == ErrMultiVolume {
			err = v.openNext()
			if err != nil {
				return nil, err
			}
		} else if err == errVolumeOrArchiveEnd {
			err = v.openNext()
			if err != nil {
				// new volume doesnt exist, assume end of archive
				if errors.Is(err, fs.ErrNotExist) {
					return nil, io.EOF
				}
				return nil, err
			}
		} else {
			return nil, err
		}
	}
}

func nextNewVolName(file string) string {
	var inDigit bool
	var m []int
	for i, c := range file {
		if c >= '0' && c <= '9' {
			if !inDigit {
				m = append(m, i)
				inDigit = true
			}
		} else if inDigit {
			m = append(m, i)
			inDigit = false
		}
	}
	if inDigit {
		m = append(m, len(file))
	}
	if l := len(m); l >= 4 {
		// More than 1 match so assume name.part###of###.rar style.
		// Take the last 2 matches where the first is the volume number.
		m = m[l-4 : l]
		if strings.Contains(file[m[1]:m[2]], ".") || !strings.Contains(file[:m[0]], ".") {
			// Didn't match above style as volume had '.' between the two numbers or didnt have a '.'
			// before the first match. Use the second number as volume number.
			m = m[2:]
		}
	}
	// extract and increment volume number
	lo, hi := m[0], m[1]
	n, err := strconv.Atoi(file[lo:hi])
	if err != nil {
		n = 0
	} else {
		n++
	}
	// volume number must use at least the same number of characters as previous volume
	vol := fmt.Sprintf("%0"+fmt.Sprint(hi-lo)+"d", n)
	return file[:lo] + vol + file[hi:]
}

func nextOldVolName(file string) string {
	// old style volume naming
	i := strings.LastIndex(file, ".")
	// get file extension
	b := []byte(file[i+1:])

	// If 2nd and 3rd character of file extension is not a digit replace
	// with "00" and ignore any trailing characters.
	if len(b) < 3 || b[1] < '0' || b[1] > '9' || b[2] < '0' || b[2] > '9' {
		return file[:i+2] + "00"
	}

	// start incrementing volume number digits from rightmost
	for j := 2; j >= 0; j-- {
		if b[j] != '9' {
			b[j]++
			break
		}
		// digit overflow
		if j == 0 {
			// last character before '.'
			b[j] = 'A'
		} else {
			// set to '0' and loop to next character
			b[j] = '0'
		}
	}
	return file[:i+1] + string(b)
}

func hasDigits(s string) bool {
	for _, c := range s {
		if c >= '0' && c <= '9' {
			return true
		}
	}
	return false
}

func fixFileExtension(file string) string {
	// check file extensions
	i := strings.LastIndex(file, ".")
	if i < 0 {
		// no file extension, add one
		return file + ".rar"
	}
	ext := strings.ToLower(file[i+1:])
	// replace with .rar for empty extensions & self extracting archives
	if ext == "" || ext == "exe" || ext == "sfx" {
		file = file[:i+1] + "rar"
	}
	return file
}

type volumeManager struct {
	dir string // current volume directory path
	opt *options

	mu    sync.Mutex
	files []string // file names for each volume
	old   bool     // uses old naming scheme
}

func (vm *volumeManager) Files() []string {
	vm.mu.Lock()
	defer vm.mu.Unlock()
	return vm.files
}

func (vm *volumeManager) tryNewName(file string) (fs.File, error) {
	// try using new naming scheme
	name := nextNewVolName(file)
	f, err := vm.opt.fs.Open(vm.dir + name)
	if !errors.Is(err, fs.ErrNotExist) {
		vm.files = append(vm.files, name)
		return f, err
	}
	// file didn't exist, try old naming scheme
	name = nextOldVolName(file)
	f, oldErr := vm.opt.fs.Open(vm.dir + name)
	if !errors.Is(oldErr, fs.ErrNotExist) {
		vm.old = true
		vm.files = append(vm.files, name)
		return f, oldErr
	}
	return nil, err
}

// next opens the next volume file in the archive.
func (vm *volumeManager) openVolumeFile(volnum int) (fs.File, error) {
	vm.mu.Lock()
	defer vm.mu.Unlock()

	var file string
	// check for cached volume name
	if volnum < len(vm.files) {
		return vm.opt.fs.Open(vm.dir + vm.files[volnum])
	}
	file = vm.files[len(vm.files)-1]
	if len(vm.files) == 1 {
		file = fixFileExtension(file)
		if !vm.old && hasDigits(file) {
			return vm.tryNewName(file)
		}
		vm.old = true
	}
	for len(vm.files) <= volnum {
		if vm.old {
			file = nextOldVolName(file)
		} else {
			file = nextNewVolName(file)
		}
		vm.files = append(vm.files, file)
	}
	return vm.opt.fs.Open(vm.dir + file)
}

func (vm *volumeManager) newVolume(volnum int) (*fileVolume, error) {
	f, err := vm.openVolumeFile(volnum)
	if err != nil {
		return nil, err
	}
	v, err := newVolume(f, vm.opt, volnum)
	if err != nil {
		f.Close()
		return nil, err
	}
	mv := &fileVolume{
		readerVolume: v,
		f:            f,
		vm:           vm,
	}
	return mv, nil
}

func (vm *volumeManager) openBlockOffset(h *fileBlockHeader, offset int64) (*fileVolume, error) {
	v, err := vm.newVolume(h.volnum)
	if err != nil {
		return nil, err
	}
	if h.dataOff < v.br.off {
		v.Close()
		return nil, ErrInvalidHeaderOff
	}
	err = v.br.Discard(h.dataOff - v.br.off + offset)
	v.n = h.PackedSize - offset
	if err != nil {
		v.Close()
		return nil, err
	}
	return v, nil
}

func (vm *volumeManager) openArchiveFile(blocks *fileBlockList) (fs.File, error) {
	h := blocks.firstBlock()
	if h.Solid {
		return nil, ErrSolidOpen
	}
	v, err := vm.openBlockOffset(h, 0)
	if err != nil {
		return nil, err
	}
	pr := newPackedFileReader(v, vm.opt)
	f, err := pr.newArchiveFile(blocks)
	if err != nil {
		v.Close()
		return nil, err
	}
	if sr, ok := f.(archiveFileSeeker); ok {
		return &fileSeekCloser{archiveFileSeeker: sr, Closer: v}, nil
	}
	return &fileCloser{archiveFile: f, Closer: v}, nil
}

func openVolume(filename string, opts *options) (*fileVolume, error) {
	dir, file := filepath.Split(filename)
	vm := &volumeManager{
		dir:   dir,
		files: []string{file},
		opt:   opts,
	}
	v, err := vm.newVolume(0)
	if err != nil {
		return nil, err
	}
	vm.old = v.arc.useOldNaming()
	return v, nil
}
