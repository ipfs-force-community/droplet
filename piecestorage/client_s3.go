package piecestorage

import (
	"context"
	"fmt"
	"github.com/filecoin-project/venus-market/utils"
	"io"
	"net/http"
)

var _ IPieceStorage = (*ClientS3Storage)(nil)

type ClientS3Storage struct {
	presignUrl IPreSignOp
}

func (c ClientS3Storage) Type() string {
	panic("implement me")
}

func (c ClientS3Storage) SaveTo(ctx context.Context, s string, reader io.Reader) (int64, error) {
	counterR := utils.NewCounterBufferReader(reader)
	writeUrl, err := c.presignUrl.GetWriteUrl(ctx, s)
	if err != nil {
		return 0, err
	}

	req, err := http.NewRequest("PUT", writeUrl, counterR)
	if err != nil {
		fmt.Println("error creating request", writeUrl)
		return 0, nil
	}

	_, err = http.DefaultClient.Do(req)
	if err != nil {
		fmt.Println("failed making request")
		return 0, err
	}
	return int64(counterR.Count()), nil
}

func (c ClientS3Storage) Read(ctx context.Context, s string) (io.ReadCloser, error) {
	readUrl, err := c.presignUrl.GetReadUrl(ctx, s)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest("GET", readUrl, nil)
	if err != nil {
		return nil, err
	}

	resp, err := http.DefaultClient.Do(req.WithContext(ctx))
	if err != nil {
		return nil, err
	}
	return resp.Body, nil
}

func (c ClientS3Storage) Len(ctx context.Context, string2 string) (int64, error) {
	panic("implement me")
}

func (c ClientS3Storage) ReadOffset(ctx context.Context, s string, i int, i2 int) (io.ReadCloser, error) {
	panic("implement me")
}

func (c ClientS3Storage) Has(ctx context.Context, s string) (bool, error) {
	panic("implement me")
}

func (c ClientS3Storage) Validate(s string) error {
	if c.presignUrl == nil {
		return fmt.Errorf("client s3 storage must has presign url")
	}
	return nil
}

func (c ClientS3Storage) GetReadUrl(ctx context.Context, s2 string) (string, error) {
	return c.presignUrl.GetReadUrl(ctx, s2)
}

func (c ClientS3Storage) GetWriteUrl(ctx context.Context, s2 string) (string, error) {
	return c.presignUrl.GetWriteUrl(ctx, s2)
}
