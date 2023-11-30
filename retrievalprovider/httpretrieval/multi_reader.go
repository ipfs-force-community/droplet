package httpretrieval

import (
	"errors"
	"io"

	"github.com/filecoin-project/go-padreader"
)

var errSeeker = errors.New("seeker can't seek")
var errWhence = errors.New("seek: invalid whence")

type multiReader struct {
	reader       io.ReadSeeker
	readerSize   uint64
	readerOffset int

	nullReader       io.Reader
	nullReaderSize   uint64
	nullReaderOffset int
}

func newMultiReader(r io.ReadSeeker, size uint64) *multiReader {
	padSize := padreader.PaddedSize(size)
	nullReaderSize := uint64(padSize) - size
	return &multiReader{
		reader:     r,
		readerSize: size,

		nullReader:     io.LimitReader(nullReader{}, int64(nullReaderSize)),
		nullReaderSize: nullReaderSize,
	}
}

func (mr *multiReader) Read(p []byte) (int, error) {
	if int(mr.readerSize)-mr.readerOffset >= len(p) {
		n, err := mr.reader.Read(p)
		mr.readerOffset += n
		return n, err
	}

	var n int
	var err error
	remain := int(mr.readerSize) - mr.readerOffset
	if remain > 0 {
		n, err = mr.reader.Read(p[:remain])
		mr.readerOffset += n
		if err != nil {
			return n, err
		}
	}

	remain = int(mr.nullReaderSize) - mr.nullReaderOffset
	if remain <= 0 {
		return 0, io.EOF
	}
	if len(p)-n > remain {
		n2, err := mr.nullReader.Read(p[n : remain+n])
		mr.nullReaderOffset += n2
		return n + n2, err
	}

	n2, err := mr.nullReader.Read(p[n:])
	mr.nullReaderOffset += n2

	return n + n2, err
}

func (mr *multiReader) Seek(offset int64, whence int) (int64, error) {
	switch whence {
	default:
		return 0, errWhence
	case io.SeekStart:
		seekOffset2 := 0
		seekOffset := offset
		if offset > int64(mr.readerSize) {
			seekOffset = int64(mr.readerSize)
			seekOffset2 = int(offset - int64(mr.readerSize))
		}
		_, err := mr.reader.Seek(seekOffset, whence)
		if err != nil {
			return 0, errSeeker
		}
		mr.readerOffset = int(seekOffset)
		mr.nullReaderOffset = seekOffset2
		return offset, nil
	case io.SeekCurrent:
		if offset == 0 {
			return 0, nil
		}

		if offset > 0 {
			if mr.readerSize > uint64(mr.readerOffset) {
				remain := int64(mr.readerSize) - int64(mr.readerOffset)
				seekOffset := offset
				if offset > remain {
					seekOffset = remain
					mr.nullReaderOffset = int(offset) - int(remain)
				}
				_, err := mr.reader.Seek(seekOffset, whence)
				if err != nil {
					return 0, errSeeker
				}
				return offset, nil
			}
			mr.nullReaderOffset = +int(offset)
			return offset, nil
		}
		if offset+int64(mr.nullReaderOffset) < 0 {
			mr.nullReaderOffset = 0
			mr.readerOffset = int(offset) + mr.nullReaderOffset + int(mr.readerSize)
			_, err := mr.reader.Seek(offset+int64(mr.nullReaderOffset), whence)
			if err != nil {
				return 0, errSeeker
			}
			return offset, nil
		}
		mr.nullReaderOffset += int(offset)

		return offset, nil
	case io.SeekEnd:
		_, err := mr.reader.Seek(0, whence)
		if err != nil {
			return 0, err
		}
		mr.readerOffset = int(mr.readerSize)
		mr.nullReaderOffset = int(mr.nullReaderSize)
		return int64(mr.readerSize) + int64(mr.nullReaderSize) + offset, nil
	}
}

var _ io.ReadSeeker = &multiReader{}

type nullReader struct{}

// Read writes NUL bytes into the provided byte slice.
func (nr nullReader) Read(b []byte) (int, error) {
	for i := range b {
		b[i] = 0
	}
	return len(b), nil
}
