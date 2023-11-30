package httpretrieval

import (
	"crypto/rand"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/filecoin-project/go-padreader"

	"github.com/stretchr/testify/assert"
)

func TestMultiReader(t *testing.T) {
	dir := t.TempDir()
	size := 10
	paddedSize := int(padreader.PaddedSize(uint64(size)))
	buf := make([]byte, size)
	_, err := rand.Read(buf)
	assert.NoError(t, err)
	f, err := os.Create(filepath.Join(dir, "test"))
	assert.NoError(t, err)
	_, err = f.Write(buf)
	assert.NoError(t, err)
	defer f.Close() // nolint

	_, err = f.Seek(0, io.SeekStart)
	assert.NoError(t, err)
	r := newMultiReader(f, uint64(size))
	buf2 := make([]byte, 2*size)
	n, err := r.Read(buf2)
	assert.NoError(t, err)
	assert.Equal(t, 2*size, n)
	assert.Equal(t, buf, buf2[:size])
	assert.Equal(t, make([]byte, size), buf2[size:])

	_, err = f.Seek(0, io.SeekStart)
	assert.NoError(t, err)
	r = newMultiReader(f, uint64(size))
	buf2 = make([]byte, size)
	n, err = r.Read(buf2)
	assert.NoError(t, err)
	assert.Equal(t, size, n)
	assert.Equal(t, buf, buf2)

	_, err = f.Seek(0, io.SeekStart)
	assert.NoError(t, err)
	r = newMultiReader(f, uint64(size))
	buf2 = make([]byte, size*100)
	n, err = r.Read(buf2)
	assert.NoError(t, err)
	assert.Equal(t, paddedSize, n)
	assert.Equal(t, buf, buf2[:size])
	assert.Equal(t, make([]byte, paddedSize-size), buf2[size:paddedSize])
}

func TestMultiReaderSeek(t *testing.T) {
	dir := t.TempDir()
	size := 10
	paddedSize := int(padreader.PaddedSize(uint64(size)))
	buf := make([]byte, size)
	_, err := rand.Read(buf)
	assert.NoError(t, err)
	f, err := os.Create(filepath.Join(dir, "test"))
	assert.NoError(t, err)
	_, err = f.Write(buf)
	assert.NoError(t, err)
	defer f.Close() // nolint

	_, err = f.Seek(0, io.SeekStart)
	assert.NoError(t, err)
	r := newMultiReader(f, uint64(size))

	var zero int64
	ret, err := r.Seek(zero, io.SeekStart)
	assert.NoError(t, err)
	assert.Equal(t, zero, ret)

	ret, err = r.Seek(zero, io.SeekEnd)
	assert.NoError(t, err)
	assert.Equal(t, int64(paddedSize), ret)

	ret, err = r.Seek(zero, io.SeekCurrent)
	assert.NoError(t, err)
	assert.Equal(t, zero, ret)

	for _, offset := range []int{1, 5, 10, 15, 50, paddedSize, 200} {
		buf2 := make([]byte, size)
		r = newMultiReader(f, uint64(size))

		ret, err = r.Seek(int64(offset), io.SeekStart)
		assert.NoError(t, err)
		assert.Equal(t, int64(offset), ret)

		n, err := r.Read(buf2)
		if offset >= paddedSize {
			assert.Equal(t, io.EOF, err)
			assert.Equal(t, 0, n)
			continue
		}
		assert.NoError(t, err)
		assert.Equal(t, size, n)
		if offset <= size {
			assert.Equal(t, buf[offset:size], buf2[:size-offset])
			assert.Equal(t, make([]byte, offset), buf2[n-offset:])
		} else {
			assert.Equal(t, make([]byte, size), buf2)
		}
	}

	// todo: test r.Seek(zero, io.SeekCurrent)
}
