package piecestorage

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"strings"
	"time"

	logging "github.com/ipfs/go-log/v2"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"

	"github.com/filecoin-project/dagstore/mount"
	"github.com/filecoin-project/venus-market/v2/config"
	"github.com/filecoin-project/venus-market/v2/utils"
)

var log = logging.Logger("piece-storage")

func parseS3Endpoint(endPoint string) (string, string, string, error) {
	endPointUrl, err := url.Parse(endPoint)
	if err != nil {
		return "", "", "", fmt.Errorf("parser s3 endpoint %s %w", endPoint, err)
	}

	hostSeq := strings.Split(endPointUrl.Host, ".")
	if len(hostSeq) < 2 {
		return "", "", "", fmt.Errorf("must specify region in host %s", endPoint)
	}

	if endPointUrl.Path == "" {
		return "", "", "", fmt.Errorf("must append bucket in endpoint %s", endPoint)
	}
	bucket := strings.Trim(endPointUrl.Path, "/")

	endPointUrl.Path = ""
	return endPointUrl.String(), hostSeq[0], bucket, nil
}

type s3PieceStorage struct {
	bucket   string
	s3Client *s3.S3
	uploader *s3manager.Uploader
	s3Cfg    *config.S3PieceStorage
}

func newS3PieceStorage(s3Cfg *config.S3PieceStorage) (IPieceStorage, error) {
	endpoint, region, bucket, err := parseS3Endpoint(s3Cfg.EndPoint)
	if err != nil {
		return nil, err
	}
	sess := session.Must(session.NewSession(&aws.Config{
		Credentials:      credentials.NewStaticCredentials(s3Cfg.AccessKey, s3Cfg.SecretKey, s3Cfg.Token),
		Endpoint:         aws.String(endpoint),
		S3ForcePathStyle: aws.Bool(false),
		Region:           aws.String(region),
		//LogLevel:         aws.LogLevel(aws.LogDebug),
	}))
	uploader := s3manager.NewUploader(sess, func(uploader *s3manager.Uploader) {
		uploader.Concurrency = 8
	})
	return &s3PieceStorage{s3Cfg: s3Cfg, bucket: bucket, s3Client: s3.New(sess), uploader: uploader}, nil
}

func (s *s3PieceStorage) SaveTo(ctx context.Context, resourceId string, r io.Reader) (int64, error) {
	if s.s3Cfg.ReadOnly {
		return 0, fmt.Errorf("do not write to a 'readonly' piece store")
	}

	countReader := utils.NewCounterBufferReader(r)
	resp, err := s.uploader.Upload(&s3manager.UploadInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(resourceId),
		Body:   countReader,
	})
	if err != nil {
		return 0, err
	}
	log.Infof("update file to s3 piece storage, upload id %s", resp.UploadID)
	return int64(countReader.Count()), nil
}

func (s *s3PieceStorage) Len(ctx context.Context, piececid string) (int64, error) {
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

func (s *s3PieceStorage) ListResourceIds(ctx context.Context) ([]string, error) {
	params := &s3.ListObjectsV2Input{
		Bucket: aws.String(s.bucket),
	}

	result, err := s.s3Client.ListObjectsV2(params)
	if err != nil {
		return nil, err
	}
	var pieces []string
	for _, obj := range result.Contents {
		var name = *obj.Key
		if name[len(name)-1] != '/' && obj.Size != nil && *obj.Size != 0 {
			pieces = append(pieces, name)
		}
	}
	return pieces, nil
}

func (s s3PieceStorage) GetReaderCloser(ctx context.Context, resourceId string) (io.ReadCloser, error) {
	params := &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(resourceId),
	}

	result, err := s.s3Client.GetObject(params)
	if err != nil {
		return nil, err
	}
	return result.Body, nil
}

func (s s3PieceStorage) GetMountReader(ctx context.Context, resourceId string) (mount.Reader, error) {
	len, err := s.Len(ctx, resourceId)
	if err != nil {
		return nil, err
	}
	return newSeekWraper(s.s3Client, s.bucket, resourceId, len-1), nil
}

func (s s3PieceStorage) GetRedirectUrl(ctx context.Context, resourceId string) (string, error) {
	if has, err := s.Has(ctx, resourceId); err != nil {
		return "", fmt.Errorf("check object: %s exist error:%w", resourceId, err)
	} else if !has {
		return "", fmt.Errorf("object: %s not exists", resourceId)
	}

	params := &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(resourceId),
	}

	req, _ := s.s3Client.GetObjectRequest(params)
	return req.Presign(time.Hour * 24)
}

func (s *s3PieceStorage) CanAllocate(size int64) bool {
	return true
}

func (s *s3PieceStorage) Has(ctx context.Context, piececid string) (bool, error) {
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

func (s *s3PieceStorage) Validate(piececid string) error {
	_, err := s.s3Client.GetBucketAcl(&s3.GetBucketAclInput{
		Bucket: aws.String(s.bucket),
	})
	return err
}

func (s *s3PieceStorage) Type() Protocol {
	return S3
}

func (s *s3PieceStorage) ReadOnly() bool {
	return s.s3Cfg.ReadOnly
}

func (s *s3PieceStorage) GetName() string {
	return s.s3Cfg.Name
}

var _ mount.Reader = (*seekWraper)(nil)

type seekWraper struct {
	s3Client   *s3.S3
	bucket     string
	resourceId string
	len        int64
	offset     int64
}

func newSeekWraper(s3Client *s3.S3, bucket, resourceId string, len int64) *seekWraper {
	return &seekWraper{s3Client, bucket, resourceId, len, 0}
}

func (sw *seekWraper) Read(p []byte) (n int, err error) {
	params := &s3.GetObjectInput{
		Bucket: aws.String(sw.bucket),
		Key:    aws.String(sw.resourceId),
		Range:  aws.String(fmt.Sprintf("bytes=%d-%d", sw.offset, sw.offset+int64(len(p)))),
	}

	result, err := sw.s3Client.GetObject(params)
	if err != nil {
		return 0, err
	}
	n, err = result.Body.Read(p)
	if err != nil {
		return 0, err
	}
	sw.offset = sw.offset + int64(n)
	return
}

func (sw *seekWraper) Seek(offset int64, whence int) (int64, error) {
	if whence != io.SeekStart {
		return 0, fmt.Errorf("only support seek from start for oss")
	}
	sw.offset = offset
	return sw.offset, nil
}

func (sw *seekWraper) ReadAt(p []byte, off int64) (n int, err error) {
	maxLen := off + int64(len(p))
	if maxLen > sw.len {
		maxLen = sw.len
	}
	params := &s3.GetObjectInput{
		Bucket: aws.String(sw.bucket),
		Key:    aws.String(sw.resourceId),
		Range:  aws.String(fmt.Sprintf("bytes=%d-%d", off, maxLen)),
	}

	req, result := sw.s3Client.GetObjectRequest(params)
	err = req.Send()
	if err != nil {
		return 0, err
	}
	return io.ReadFull(result.Body, p)
}

func (sw *seekWraper) Close() error {
	return nil
}
