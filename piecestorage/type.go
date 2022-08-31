package piecestorage

import (
	"context"
	"fmt"
	"io"

	"github.com/filecoin-project/venus/venus-shared/types/market"

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

type IPieceStorage interface {
	Type() Protocol
	ReadOnly() bool
	GetName() string
	SaveTo(context.Context, string, io.Reader) (int64, error)
	Len(context.Context, string) (int64, error)
	//ListResourceIds get resource ids from piece store
	ListResourceIds(ctx context.Context) ([]string, error)
	//GetMountReader use direct read if storage have low performance effecitive ReadAt
	GetReaderCloser(context.Context, string) (io.ReadCloser, error)
	//GetMountReader used to support dagstore
	GetMountReader(context.Context, string) (mount.Reader, error)
	//GetRedirectUrl get url if storage support redirect
	GetRedirectUrl(context.Context, string) (string, error)
	Has(context.Context, string) (bool, error)
	Validate(string) error
	GetStorageStatus() (market.StorageStatus, error)
}
