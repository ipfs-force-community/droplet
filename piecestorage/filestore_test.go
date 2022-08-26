package piecestorage

import (
	"context"
	"crypto/rand"
	"io"
	"os"
	path2 "path"
	"testing"

	"github.com/google/uuid"

	"github.com/filecoin-project/venus-market/v2/config"
	"github.com/stretchr/testify/require"
)

func TestReWrite(t *testing.T) {
	r := io.LimitReader(rand.Reader, 100)
	path := path2.Join(os.TempDir(), uuid.New().String())
	_ = os.MkdirAll(path, os.ModePerm)
	name := "market-test-tmp"
	filepath := path2.Join(path, name)
	_ = os.Remove(filepath)

	ctx := context.TODO()
	ifs, err := NewFsPieceStorage(&config.FsPieceStorage{ReadOnly: false, Path: path})
	require.NoErrorf(t, err, "open file storage")
	wlen, err := ifs.SaveTo(ctx, name, r)

	require.NoErrorf(t, err, "expect to write file ")
	require.Equal(t, wlen, int64(100))
	fs, err := os.Open(filepath)
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
	require.Len(t, resources, 1)
	require.Equal(t, resources[0], name)

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
