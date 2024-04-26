package piecestorage

import (
	"context"
	"crypto/rand"
	"io"
	"os"
	path2 "path"
	"testing"

	"github.com/google/uuid"

	"github.com/ipfs-force-community/droplet/v2/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReWrite(t *testing.T) {
	r := io.LimitReader(rand.Reader, 100)
	path := path2.Join(os.TempDir(), uuid.New().String())
	_ = os.MkdirAll(path, os.ModePerm)
	name := "market-test-tmp"
	filePath := path2.Join(path, name)
	_ = os.Remove(filePath)
	defer os.RemoveAll(path) // nolint

	ctx := context.TODO()
	ifs, err := NewFsPieceStorage(&config.FsPieceStorage{ReadOnly: false, Path: path})
	require.NoErrorf(t, err, "open file storage")
	wlen, err := ifs.SaveTo(ctx, name, r)

	require.NoErrorf(t, err, "expect to write file ")
	require.Equal(t, wlen, int64(100))
	fs, err := os.Open(filePath)
	if err != nil {
		if !os.IsExist(err) {
			require.NoErrorf(t, err, "expect file exit")
		}
	}

	linkPath := path2.Join(path, name+".car")
	assert.NoError(t, os.Symlink(filePath, linkPath))
	l, err := ifs.Len(ctx, name+".car")
	assert.NoError(t, err)
	assert.Equal(t, int64(100), l)

	buf, err := io.ReadAll(fs)
	require.NoErrorf(t, err, "expect read file success")
	if len(buf) != 100 {
		require.Equal(t, int64(len(buf)), int64(100))
	}

	noExistFile := "f111"
	has, err := ifs.Has(ctx, noExistFile)
	require.NoError(t, err)
	require.False(t, has)

	length, err := ifs.Len(ctx, noExistFile)
	require.Error(t, err)
	assert.Equal(t, int64(0), length)

	readerCloser, err := ifs.GetReaderCloser(ctx, noExistFile)
	require.Error(t, err)
	assert.Nil(t, readerCloser)

	mounterReader, err := ifs.GetMountReader(ctx, noExistFile)
	require.Error(t, err)
	assert.Nil(t, mounterReader)
}
