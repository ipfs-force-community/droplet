package utils

import "io"

// NewLimitedBufferReader returns a reader that reads from the given reader
// but limits the amount of data returned to at most n bytes.
func NewLimitedBufferReader(r io.ReadCloser, n int) io.ReadCloser {
	return &limitedBufferReader{
		r: r,
		n: n,
	}
}

type limitedBufferReader struct {
	r io.ReadCloser
	n int
}

func (r *limitedBufferReader) Close() error {
	return r.r.Close()
}

func (r *limitedBufferReader) Read(p []byte) (n int, err error) {
	np := p
	if len(np) > r.n {
		np = np[:r.n]
	}
	return r.r.Read(np)
}
