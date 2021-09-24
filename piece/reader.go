package piece

import (
	"golang.org/x/xerrors"
	"io"
	"os"
	"strings"
)

func Read(path string) (io.ReadCloser, error) {
	pieceStorage := strings.Split(path, ":")
	if len(pieceStorage) != 2 {
		return nil, xerrors.Errorf("wrong format for piece storage %w", path)
	}
	switch pieceStorage[0] {
	case "fs":
		return os.Open(path)
	default:
		return nil, xerrors.Errorf("unsupport piece piecestorage type %s", path)
	}
}

func ReadOffset(path string, offset, size int) (io.ReadCloser, error) {
	pieceStorage := strings.Split(path, ":")
	if len(pieceStorage) != 2 {
		return nil, xerrors.Errorf("wrong format for piece storage %w", path)
	}
	switch pieceStorage[0] {
	case "fs":
		fs, err := os.Open(pieceStorage[1])
		if err != nil {
			return nil, xerrors.Errorf("unable to open file %s %w", pieceStorage, err)
		}
		_, err = fs.Seek(int64(offset), 0)
		if err != nil {
			return nil, xerrors.Errorf("unable to seek position to %din file %s %w", offset, pieceStorage, err)
		}
		return NewLimitedBufferReader(fs, int(size)), nil
	default:
		return nil, xerrors.Errorf("unsupport piece piecestorage type %s", path)
	}
}
