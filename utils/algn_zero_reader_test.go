package utils

import (
	"bytes"
	"io"
	"io/ioutil"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAlignZeroReader(t *testing.T) {
	getPayloadReader := func(size int) reader {
		payloadBytes := make([]byte, size)
		for i := 0; i < size; i++ {
			payloadBytes[i] = 1
		}
		r := bytes.NewReader(payloadBytes)
		return &WrapCloser{r, r}
	}

	payloadSize := 5
	size := 10
	testCases := []struct {
		seek            int
		readCount       int
		expectOneNumber int
		expectErr       error
	}{
		{
			1, 2, 2, nil,
		},
		{
			1, 4, 4, nil,
		},
		{
			1, 5, 4, nil,
		},
		{
			1, 9, 4, io.EOF,
		},
		{
			1, 10, 4, io.EOF,
		},
		{
			4, 2, 1, nil,
		},
		{
			5, 2, 0, nil,
		},
		{
			6, 2, 0, nil,
		},
		{
			10, 2, 0, io.EOF,
		},
	}

	for _, tcase := range testCases {
		r := getPayloadReader(payloadSize)
		algnR := NewAlgnZeroMountReader(r, payloadSize, size)

		toRead := make([]byte, tcase.readCount)
		_, err := algnR.Seek(int64(tcase.seek), io.SeekStart)
		assert.Nil(t, err)
		rLen, err := algnR.Read(toRead)
		assert.Equal(t, tcase.expectErr, err)
		if err == io.EOF {
			continue
		}
		assert.Equal(t, tcase.readCount, rLen)
		countOne := 0
		for i := 0; i < len(toRead); i++ {
			countOne = countOne + int(toRead[i])
		}
		assert.Equal(t, tcase.expectOneNumber, countOne)
	}

}

func TestReadALl(t *testing.T) {
	getPayloadReader := func(size int) reader {
		payloadBytes := make([]byte, size)
		for i := 0; i < size; i++ {
			payloadBytes[i] = 1
		}
		r := bytes.NewReader(payloadBytes)
		return &WrapCloser{r, r}
	}

	payloadSize := 5
	size := 1024 * 1024

	r := getPayloadReader(payloadSize)
	algnR := NewAlgnZeroMountReader(r, payloadSize, size)
	p, err := ioutil.ReadAll(algnR)
	assert.Nil(t, err)
	countOne := 0
	for i := 0; i < len(p); i++ {
		countOne = countOne + int(p[i])
	}
	assert.Equal(t, payloadSize, countOne)
	assert.Equal(t, size, len(p))
}

func TestReadAt(t *testing.T) {
	getPayloadReader := func(size int) reader {
		payloadBytes := make([]byte, size)
		for i := 0; i < size; i++ {
			payloadBytes[i] = 1
		}
		r := bytes.NewReader(payloadBytes)
		return &WrapCloser{r, r}
	}

	payloadSize := 5
	size := 10
	testCases := []struct {
		offset          int64
		readCount       int
		expectOneNumber int
		expectErr       error
	}{
		{
			1, 2, 2, nil,
		},
		{
			1, 4, 4, nil,
		},
		{
			1, 5, 4, nil,
		},
		{
			1, 9, 4, io.EOF,
		},
		{
			1, 10, 4, io.EOF,
		},
		{
			4, 2, 1, nil,
		},
		{
			5, 2, 0, nil,
		},
		{
			6, 2, 0, nil,
		},
		{
			10, 2, 0, io.EOF,
		},
	}

	for _, tcase := range testCases {
		r := getPayloadReader(payloadSize)
		algnR := NewAlgnZeroMountReader(r, payloadSize, size)

		toRead := make([]byte, tcase.readCount)
		rLen, err := algnR.ReadAt(toRead, tcase.offset)
		assert.Equal(t, tcase.expectErr, err)
		if err == io.EOF {
			continue
		}
		assert.Equal(t, tcase.readCount, rLen)
		countOne := 0
		for i := 0; i < len(toRead); i++ {
			countOne = countOne + int(toRead[i])
		}
		assert.Equal(t, tcase.expectOneNumber, countOne)
	}
}
