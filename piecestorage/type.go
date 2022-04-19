package piecestorage

import (
	"context"
	"io"
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
	Read(context.Context, string) (io.ReadCloser, error)
	Len(ctx context.Context, string2 string) (int64, error)
	ReadOffset(context.Context, string, int, int) (io.ReadCloser, error)
	Has(context.Context, string) (bool, error)
	Validate(s string) error
	CanAllocate(size int64) bool
}
