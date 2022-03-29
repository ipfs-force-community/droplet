package piecestorage

import (
	"context"
	"crypto/rand"
	"io"
	"os"
	path2 "path"
	"testing"

	"github.com/filecoin-project/venus-market/config"
	"github.com/stretchr/testify/require"
)

func TestReWrite(t *testing.T) {
	r := io.LimitReader(rand.Reader, 100)
	path := os.TempDir()
	name := "market-test-tmp"
	filepath := path2.Join(path, name)
	_ = os.Remove(filepath)

	ctx := context.TODO()
	ifs, err := newFsPieceStorage(config.FsPieceStorage{Enable: true, Path: path})
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
}
