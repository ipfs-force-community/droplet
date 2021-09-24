package piece

import (
	"golang.org/x/xerrors"
	"io"
	"os"
	"strings"
)

func Read(path string) (io.ReadCloser, error) {
	pieceFile := strings.Split(path, ":")
	if len(pieceFile) != 2 {
		return nil, xerrors.Errorf("wrong format for piece storage %w", path)
	}
	switch pieceFile[0] {
	case "fs":
		return os.Open(path)
	default:
		return nil, xerrors.Errorf("unsupport piece piecestorage type %s", path)
	}
}

func ReadOffset(path string, offset, size int) (io.ReadCloser, error) {
	pieceFile := strings.Split(path, ":")
	if len(pieceFile) != 2 {
		return nil, xerrors.Errorf("wrong format for piece storage %w", path)
	}
	switch pieceFile[0] {
	case "fs":
		fs, err := os.Open(pieceFile[1])
		if err != nil {
			return nil, xerrors.Errorf("unable to open file %s %w", pieceFile, err)
		}
		_, err = fs.Seek(int64(offset), 0)
		if err != nil {
			return nil, xerrors.Errorf("unable to seek position to %din file %s %w", offset, pieceFile, err)
		}
		return NewLimitedBufferReader(fs, int(size)), nil
	default:
		return nil, xerrors.Errorf("unsupport piece piecestorage type %s", path)
	}
}

func ReWrite(path string, r io.Reader) (int64, error) {
	pieceFile := strings.Split(path, ":")
	if len(pieceFile) != 2 {
		return -1, xerrors.Errorf("wrong format for piece storage %w", path)
	}
	switch pieceFile[0] {
	case "fs":
		fs, err := os.Create(pieceFile[1])
		if err != nil {
			return -1, xerrors.Errorf("unbale to create file %s, %w", pieceFile[1], err)
		}
		wlen, err := io.Copy(fs, r)
		if err != nil {
			return -1, xerrors.Errorf("unable to write file to %s %w", pieceFile[1], err)
		}
		return wlen, nil
	default:
		return -1, xerrors.Errorf("unsupport piece piecestorage type %s", path)
	}
}

func Has(path string) (bool, error) {
	pieceFile := strings.Split(path, ":")
	if len(pieceFile) != 2 {
		return false, xerrors.Errorf("wrong format for piece storage %w", path)
	}
	switch pieceFile[0] {
	case "fs":
		_, err := os.Stat(pieceFile[1])
		if err != nil {
			if os.IsNotExist(err) {
				return false, nil
			}
			return false, err
		}
		return true, nil
	default:
		return false, xerrors.Errorf("unsupport piece piecestorage type %s", path)
	}
}

func CheckValidate(path string) error {
	pieceStorage := strings.Split(path, ":")
	if len(pieceStorage) != 2 {
		return xerrors.Errorf("wrong format for piece storage %w", path)
	}
	switch pieceStorage[0] {
	case "fs":
		st, err := os.Stat(pieceStorage[1])
		if err != nil {
			return err
		}

		if !st.IsDir() {
			return xerrors.Errorf("expect a directory but got file")
		}
		return nil
	default:
		return xerrors.Errorf("unsupport piece piecestorage type %s", path)
	}
}
