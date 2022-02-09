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
	"net/url"
	"strings"
	"time"
)

var log = logging.Logger("piece-storage")

func ParserS3(dsn string) (config.S3PieceStorage, error) {
	//todo s3 dsn  s3:{access key}:{secret key}:{token option}@{endpoint}
	s3Seq := strings.Split(dsn, "@")
	if len(s3Seq) != 2 {
		return config.S3PieceStorage{}, fmt.Errorf("parser s3 config %s", dsn)
	}
	authStr := s3Seq[0]
	endPointUrl := s3Seq[1]

	authSeq := strings.Split(authStr, ":")
	if !(len(authSeq) == 2 || len(authSeq) == 3) {
		return config.S3PieceStorage{}, fmt.Errorf("parser s3 auth %s", authStr)
	}
	token := ""
	if len(authSeq) == 3 {
		token = authSeq[2]
	}

	_, _, _, err := ParseS3Endpoint(endPointUrl)
	if err != nil {
		return config.S3PieceStorage{}, fmt.Errorf("parser s3 endpoint %s", endPointUrl)
	}
	return config.S3PieceStorage{
		Enable:    true,
		EndPoint:  endPointUrl,
		AccessKey: authSeq[0],
		SecretKey: authSeq[1],
		Token:     token,
	}, nil
}

func ParseS3Endpoint(endPoint string) (string, string, string, error) {
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
		Credentials:      credentials.NewStaticCredentials(s3Cfg.AccessKey, s3Cfg.SecretKey, s3Cfg.Token),
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

//todo 下面presign两个方法用于给客户端使用，暂时仅仅支持对象存储。 可能需要一个更合适的抽象模式
func (s s3PieceStorage) GetReadUrl(ctx context.Context, s2 string) (string, error) {
	params := &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(s2),
	}

	req, _ := s.s3Client.GetObjectRequest(params)
	return req.Presign(time.Minute * 30)
}

func (s s3PieceStorage) GetWriteUrl(ctx context.Context, s2 string) (string, error) {
	req, _ := s.s3Client.PutObjectRequest(&s3.PutObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(s2),
	})
	return req.Presign(time.Minute * 30)
}

func (s s3PieceStorage) Validate(piececid string) error {
	_, err := s.s3Client.GetBucketAcl(&s3.GetBucketAclInput{
		Bucket: aws.String(s.bucket),
	})
	return err
}

func (s s3PieceStorage) Type() Protocol {
	return S3
}
