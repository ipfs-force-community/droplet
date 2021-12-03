package utils

import (
	"io"
)

type CounterBufferReader struct {
	r io.Reader
	c int
}

func NewCounterBufferReader(r io.Reader) *CounterBufferReader {
	return &CounterBufferReader{r: r}
}

func (r *CounterBufferReader) Count() int {
	return r.c
}

func (r *CounterBufferReader) Read(p []byte) (int, error) {
	n, err := r.r.Read(p)
	if err != nil {
		return 0, err
	}
	r.c += n
	return n, nil
}
