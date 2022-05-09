package piecestorage

import (
	"context"
	"fmt"
	"io"

	"github.com/filecoin-project/dagstore/mount"
)

var (
	ErrUnsupportRedirect = fmt.Errorf("this storage unsupport redirect url")
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
	Len(context.Context, string) (int64, error)
	//GetMountReader use direct read if storage have low performance effecitive ReadAt
	GetReaderCloser(context.Context, string) (io.ReadCloser, error)
	//GetMountReader used to support dagstore
	GetMountReader(context.Context, string) (mount.Reader, error)
	//GetRedirectUrl get url if storage support redirect
	GetRedirectUrl(context.Context, string) (string, error)
	Has(context.Context, string) (bool, error)
	Validate(string) error
	CanAllocate(int64) bool
}
