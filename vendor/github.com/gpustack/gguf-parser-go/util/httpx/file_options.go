package httpx

type SeekerFileOption struct {
	bufSize                 int
	size                    int
	skipRangeDownloadDetect bool
}

func SeekerFileOptions() *SeekerFileOption {
	return &SeekerFileOption{
		bufSize: 4 * 1024 * 1024, // 4mb
	}
}

// WithBufferSize sets the size of the buffer to read the file,
//
// Default is 4mb.
func (o *SeekerFileOption) WithBufferSize(bufSize int) *SeekerFileOption {
	if o == nil || bufSize <= 0 {
		return o
	}
	o.bufSize = bufSize
	return o
}

// WithSize sets the size of the file to read,
//
// If the size is greater than the content size of the file, it will return an error.
func (o *SeekerFileOption) WithSize(size int) *SeekerFileOption {
	if o == nil || size <= 0 {
		return o
	}
	o.size = size
	return o
}

// WithoutRangeDownloadDetect disables range download detection.
//
// Usually, OpenSeekerFile sends a "HEAD" HTTP request to destination to get the content size from the "Content-Length" header,
// and confirms whether supports range download via the "Accept-Ranges" header.
// However, some servers may not support the "HEAD" method, or the "Accept-Ranges" header is not set correctly.
//
// With this option, OpenSeekerFile sends "GET" HTTP request to get the content size as usual,
// and does not confirm whether supports range download. But during the seeking read,
// it still uses the "Range" header to read the file.
func (o *SeekerFileOption) WithoutRangeDownloadDetect() *SeekerFileOption {
	if o == nil {
		return o
	}
	o.skipRangeDownloadDetect = true
	return o
}

// If is a conditional option,
// which receives a boolean condition to trigger the given function or not.
func (o *SeekerFileOption) If(condition bool, then func(*SeekerFileOption) *SeekerFileOption) *SeekerFileOption {
	if condition {
		return then(o)
	}
	return o
}
