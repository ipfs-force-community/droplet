package piecestorage

import (
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	_ "github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/filecoin-project/venus-market/config"
	"github.com/filecoin-project/venus-market/utils"
	logging "github.com/ipfs/go-log/v2"
	"io"
)

var log = logging.Logger("piece-storage")

type s3PieceStorage struct {
	s3Cfg      config.S3PieceStorage
	bucket     string
	s3Client   *s3.S3
	uploader   *s3manager.Uploader
	downloader *s3manager.Downloader
}

func newS3PieceStorage(s3Cfg config.S3PieceStorage) (IPieceStorage, error) {
	endpoint, region, bucket, err := ParseS3Endpoint(s3Cfg.EndPoint)
	if err != nil {
		return nil, err
	}
	sess := session.Must(session.NewSession(&aws.Config{
		Credentials:      credentials.NewStaticCredentials(s3Cfg.AccessKey, s3Cfg.SecretKey, ""),
		Endpoint:         aws.String(endpoint),
		S3ForcePathStyle: aws.Bool(false),
		Region:           aws.String(region),
	}))
	uploader := s3manager.NewUploader(sess, func(uploader *s3manager.Uploader) {
		uploader.Concurrency = 8
	})
	return &s3PieceStorage{s3Cfg: s3Cfg, bucket: bucket, s3Client: s3.New(sess), uploader: uploader}, nil
}

func (s s3PieceStorage) SaveTo(ctx context.Context, s2 string, r io.Reader) (int64, error) {
	countReader := utils.NewCounterBufferReader(r)
	resp, err := s.uploader.Upload(&s3manager.UploadInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(s2),
		Body:   countReader,
	})
	if err != nil {
		return 0, err
	}
	log.Infof("update file to s3 piece storage, upload id %s", resp.UploadID)
	return int64(countReader.Count()), nil
}

func (s s3PieceStorage) Read(ctx context.Context, s2 string) (io.ReadCloser, error) {
	params := &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(s2),
	}

	result, err := s.s3Client.GetObject(params)
	if err != nil {
		return nil, err
	}
	return result.Body, nil
}

func (s s3PieceStorage) Len(ctx context.Context, piececid string) (int64, error) {
	params := &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(piececid),
	}

	result, err := s.s3Client.GetObject(params)
	if err != nil {
		return 0, err
	}
	return *result.ContentLength, nil
}

func (s s3PieceStorage) ReadOffset(ctx context.Context, s2 string, offset int, size int) (io.ReadCloser, error) {
	params := &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(s2),
		Range:  aws.String(fmt.Sprintf("bytes=%d-%d", offset, offset+size)),
	}

	result, err := s.s3Client.GetObject(params)
	if err != nil {
		return nil, err
	}
	return utils.NewLimitedBufferReader(result.Body, size), nil
}

func (s s3PieceStorage) Has(ctx context.Context, piececid string) (bool, error) {
	params := &s3.HeadObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(piececid),
	}

	_, err := s.s3Client.HeadObject(params)
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case "NotFound": // s3.ErrCodeNoSuchKey does not work, aws is missing this error code so we hardwire a string
				return false, nil
			default:
				return false, err
			}
		}
		return false, err
	}
	return true, nil
}

func (s s3PieceStorage) Validate(piececid string) error {
	_, err := s.s3Client.GetBucketAcl(&s3.GetBucketAclInput{
		Bucket: aws.String(s.bucket),
	})
	return err
}
