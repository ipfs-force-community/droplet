package piecestorage

import (
	"context"
	"io"
	"strings"

	"github.com/filecoin-project/dagstore/mount"
)

const carSuffix = ".car"

var _ IPieceStorage = (*storeWrapper)(nil)

type storeWrapper struct {
	IPieceStorage
}

func newStoreWrapper(s IPieceStorage) IPieceStorage {
	return &storeWrapper{IPieceStorage: s}
}

func extendPiece(s string) []string {
	if strings.HasSuffix(s, carSuffix) {
		return []string{strings.Split(s, carSuffix)[0], s}
	}
	return []string{s, s + carSuffix}
}

func (sw *storeWrapper) Len(ctx context.Context, s string) (int64, error) {
	var l int64
	var err error
	for _, name := range extendPiece(s) {
		l, err = sw.IPieceStorage.Len(ctx, name)
		if err == nil {
			return l, nil
		}

	}

	return l, err
}

func (sw *storeWrapper) Has(ctx context.Context, s string) (bool, error) {
	var has bool
	var err error
	for _, name := range extendPiece(s) {
		has, err = sw.IPieceStorage.Has(ctx, name)
		if err == nil && has {
			return has, nil
		}
	}

	return has, err
}

func (sw *storeWrapper) GetReaderCloser(ctx context.Context, s string) (io.ReadCloser, error) {
	var rc io.ReadCloser
	var err error
	for _, name := range extendPiece(s) {
		rc, err = sw.IPieceStorage.GetReaderCloser(ctx, name)
		if err == nil {
			return rc, nil
		}
	}

	return rc, err
}

func (sw *storeWrapper) GetMountReader(ctx context.Context, s string) (mount.Reader, error) {
	var reader mount.Reader
	var err error
	for _, name := range extendPiece(s) {
		reader, err = sw.IPieceStorage.GetMountReader(ctx, name)
		if err == nil {
			return reader, nil
		}
	}

	return reader, err
}

func (sw *storeWrapper) GetRedirectUrl(ctx context.Context, s string) (string, error) {
	var url string
	var err error
	for _, name := range extendPiece(s) {
		url, err = sw.IPieceStorage.GetRedirectUrl(ctx, name)
		if err == nil {
			return url, nil
		}
	}

	return url, err
}

func (sw *storeWrapper) Validate(s string) error {
	var err error
	for _, name := range extendPiece(s) {
		err = sw.IPieceStorage.Validate(name)
		if err == nil {
			return nil
		}
	}

	return err
}
