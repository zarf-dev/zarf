package file

import (
	"io"
	"strings"

	"github.com/gabriel-vasile/mimetype"
)

// MIMEType attempts to guess at the MIME type of a file given the contents. If there is no contents, then an empty
// string is returned. If the MIME type could not be determined and the contents are not empty, then a MIME type
// of "application/octet-stream" is returned.
func MIMEType(reader io.Reader) string {
	if reader == nil {
		return ""
	}

	s := sizer{reader: reader}

	var mTypeStr string
	mType, err := mimetype.DetectReader(&s)
	if err == nil {
		// extract the string mimetype and ignore aux information (e.g. 'text/plain; charset=utf-8' -> 'text/plain')
		mTypeStr = strings.Split(mType.String(), ";")[0]
	}

	// we may have a reader that is not nil but the observed contents was empty
	if s.size == 0 {
		return ""
	}

	return mTypeStr
}

type sizer struct {
	reader io.Reader
	size   int64
}

func (s *sizer) Read(p []byte) (int, error) {
	n, err := s.reader.Read(p)
	s.size += int64(n)
	return n, err
}
