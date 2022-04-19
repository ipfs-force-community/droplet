package piecestorage

import (
	"context"
	"io"

	"github.com/filecoin-project/dagstore/mount"
)

type Protocol string

const (
	MemStore  Protocol = "mem" //for test
	FS        Protocol = "fs"
	S3        Protocol = "s3"
	PreSignS3 Protocol = "presigns3"
)

type StorageStatus struct {
	Capacity  int64
	Available int64 // Available to use for sector storage
}

type IPieceStorage interface {
	Type() Protocol
	ReadOnly() bool
	SaveTo(context.Context, string, io.Reader) (int64, error)
	Len(ctx context.Context, string2 string) (int64, error)
	GetReaderCloser(ctx context.Context, s string) (io.ReadCloser, error)
	GetMountReader(ctx context.Context, s string) (mount.Reader, error)
	Has(context.Context, string) (bool, error)
	Validate(s string) error
	CanAllocate(size int64) bool
}
