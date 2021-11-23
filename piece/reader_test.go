package piece

import (
	"crypto/rand"
	"github.com/stretchr/testify/require"
	"io"
	"os"
	path2 "path"
	"testing"
)

func TestReWrite(t *testing.T) {
	r := io.LimitReader(rand.Reader, 100)
	path := os.TempDir()
	filePath := path2.Join(path, "market-test-tmp")
	os.Remove(filePath)
	wlen, err := ReWrite("fs:"+filePath, r)

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
}
