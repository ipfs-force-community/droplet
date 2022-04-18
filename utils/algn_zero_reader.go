package utils

import (
	"fmt"
	"io"
)

var _ io.ReadSeekCloser = (*AlgnZeroMountReader)(nil)

type AlgnZeroMountReader struct {
	r       reader
	l       int
	size    int
	seekPos int
}

type reader interface {
	io.ReadSeekCloser
	io.ReaderAt
}

func NewAlgnZeroMountReader(r reader, payload, size int) *AlgnZeroMountReader {
	return &AlgnZeroMountReader{
		r:       r,
		l:       payload,
		size:    size,
		seekPos: 0,
	}
}

//todo change better arglerithms
func (i *AlgnZeroMountReader) ReadAt(p []byte, offset int64) (n int, err error) {
	wLen := 0
	if offset < int64(i.l) {
		wLen, err = i.r.ReadAt(p, offset)
		if err != nil && err != io.EOF {
			return 0, err
		}

	}
	err = nil
	minL := len(p) + int(offset)
	if minL >= i.size {
		err = io.EOF
		minL = i.size
	}

	alignZero := minL - wLen - int(offset)
	if alignZero > 0 {
		zero := make([]byte, alignZero)
		copy(p[wLen:], zero)
	}
	n = int(int64(minL) - offset)
	return
}

func (i *AlgnZeroMountReader) Read(p []byte) (n int, err error) {
	if i.seekPos > i.size {
		return 0, io.EOF
	}

	wLen := 0
	if i.seekPos < i.l {
		_, err := i.r.Seek(int64(i.seekPos), io.SeekStart)
		if err != nil {
			return 0, err
		}
		wLen, err = i.r.Read(p)
		if err != nil && err != io.EOF {
			return 0, err
		}

	}
	err = nil
	minL := len(p) + i.seekPos
	if minL >= i.size {
		err = io.EOF
		minL = i.size
	}

	alignZero := minL - wLen - i.seekPos
	if alignZero > 0 {
		zero := make([]byte, alignZero)
		copy(p[wLen:], zero)
	}
	n = minL - i.seekPos
	i.seekPos = minL
	return
}

func (i *AlgnZeroMountReader) Close() error {
	return i.r.Close()
}

func (i *AlgnZeroMountReader) Seek(offset int64, whence int) (int64, error) {
	if whence == io.SeekCurrent {
		i.seekPos = i.seekPos + int(offset)
		return int64(i.seekPos), nil
	} else if whence == io.SeekStart {
		i.seekPos = int(offset)
		return int64(i.seekPos), nil
	}
	return 0, fmt.Errorf("only support seek")
}

type WrapCloser struct {
	io.ReadSeeker
	io.ReaderAt
}

func (WrapCloser) Close() error {
	return nil
}
