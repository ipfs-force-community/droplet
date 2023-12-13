package piecestorage

import (
	"context"
	"crypto/rand"
	"fmt"
	"io"
	"os"
	path2 "path"
	"path/filepath"
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

	buf, err := io.ReadAll(fs)
	require.NoErrorf(t, err, "expect read file success")
	if len(buf) != 100 {
		require.Equal(t, int64(len(buf)), int64(100))
	}

	files := []string{"f1", "f2", "f3", "f4"}
	data, err := io.ReadAll(io.LimitReader(rand.Reader, 100))
	require.NoError(t, err)
	for i, f := range files {
		if i%2 == 0 {
			_ = os.MkdirAll(filepath.Join(path, "tmp"), os.ModePerm)
			assert.NoError(t, os.WriteFile(filepath.Join(path, "tmp", f), data, os.ModePerm))
			continue
		}
		wlen, err := ifs.SaveTo(ctx, f, io.LimitReader(rand.Reader, 100))
		assert.NoErrorf(t, err, "expect to write file ")
		assert.Equal(t, int64(100), wlen)
	}

	for _, name := range append(files, name) {
		_, err = ifs.GetStorageStatus()
		require.Nil(t, err)
		require.Equal(t, FS, ifs.Type())

		length, err := ifs.Len(ctx, name)
		require.NoError(t, err, "fail to get length")
		require.Equal(t, int64(100), length)

		has, err := ifs.Has(ctx, name)
		require.NoError(t, err, "fail to check file exit")
		require.True(t, has)
		require.False(t, ifs.ReadOnly())

		resources, err := ifs.ListResourceIds(ctx)
		require.NoErrorf(t, err, "expect resource but got err")
		require.Len(t, resources, 5)
		require.Contains(t, resources, name)

		readerCloser, err := ifs.GetReaderCloser(ctx, name)
		require.NoError(t, err, "fail to get reader closer")
		buf, err = io.ReadAll(readerCloser)
		require.NoErrorf(t, err, "expect read file success")
		if len(buf) != 100 {
			require.Equal(t, int64(len(buf)), int64(100))
		}

		mounterReader, err := ifs.GetMountReader(ctx, name)
		require.NoError(t, err, "fail to get mount reader")
		buf, err = io.ReadAll(mounterReader)
		require.NoErrorf(t, err, "expect read file success")
		if len(buf) != 100 {
			require.Equal(t, int64(len(buf)), int64(100))
		}
	}

	dir := "tmp"
	expectErr := fmt.Errorf("resource %s expect to be a file but found directory", dir)
	has, err := ifs.Has(ctx, dir)
	require.Equal(t, expectErr, err)
	require.False(t, has)

	length, err := ifs.Len(ctx, dir)
	require.Equal(t, expectErr, err)
	assert.Equal(t, int64(0), length)

	readerCloser, err := ifs.GetReaderCloser(ctx, dir)
	require.Equal(t, expectErr, err)
	assert.Nil(t, readerCloser)

	mounterReader, err := ifs.GetMountReader(ctx, dir)
	require.Equal(t, expectErr, err)
	assert.Nil(t, mounterReader)

	noExistFile := "f111"
	has, err = ifs.Has(ctx, noExistFile)
	require.NoError(t, err)
	require.False(t, has)

	length, err = ifs.Len(ctx, noExistFile)
	require.Error(t, err)
	assert.Equal(t, int64(0), length)

	readerCloser, err = ifs.GetReaderCloser(ctx, noExistFile)
	require.Error(t, err)
	assert.Nil(t, readerCloser)

	mounterReader, err = ifs.GetMountReader(ctx, noExistFile)
	require.Error(t, err)
	assert.Nil(t, mounterReader)
}
