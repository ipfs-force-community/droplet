package piecestorage

import (
	"context"
	"math/rand"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/filecoin-project/venus-market/v2/config"
	"github.com/stretchr/testify/assert"
)

func TestParseS3Endpoint(t *testing.T) {
	positiveCase := []string{"oss-cn-shanghai.aliyuncs.com", "https://oss-cn-shanghai.aliyuncs.com", "bucketname.oss-cn-shanghai.aliyuncs.com", "https://bucketname.oss-cn-shanghai.aliyuncs.com", "https://bucketname.oss-cn-shanghai.aliyuncs.com/dir1/dir2"}

	for _, Endpoint := range positiveCase {
		endpoint, region, err := parseS3Endpoint(Endpoint, "bucketname")
		assert.NoError(t, err)
		assert.Equal(t, "oss-cn-shanghai.aliyuncs.com", endpoint)
		assert.Equal(t, "oss-cn-shanghai", region)
	}

	negativeCase := []string{
		"bucketname.oss-cn-shanghai.aliyuncs.com/",
	}

	for _, Endpoint := range negativeCase {
		endpoint, region, err := parseS3Endpoint(Endpoint, "bucketname")
		assert.Error(t, err)
		assert.Equal(t, "", endpoint)
		assert.Equal(t, "", region)
	}

}

func TestS3PieceStorage(t *testing.T) {
	key := os.Getenv("AWS_ACCESS_KEY_ID")
	secret := os.Getenv("AWS_SECRET_ACCESS_KEY")
	bucket := os.Getenv("AWS_BUCKET")
	endpoint := os.Getenv("AWS_EndPoint")

	if key == "" || secret == "" || bucket == "" {
		t.Skip("AWS_ACCESS_KEY_ID , AWS_SECRET_ACCESS_KEY ,AWS_EndPoint and AWS_BUCKET must be set in env")
	}

	s3Cfg := &config.S3PieceStorage{
		EndPoint:  endpoint,
		Bucket:    bucket,
		AccessKey: key,
		SecretKey: secret,
		Token:     "",
		SubDir:    "",
	}
	testS3PieceStorage(t, s3Cfg)
	s3Cfg.SubDir = "test"
	testS3PieceStorage(t, s3Cfg)

}

func testS3PieceStorage(t *testing.T, s3Cfg *config.S3PieceStorage) {
	ctx := context.Background()
	endpoint, region, err := parseS3Endpoint(s3Cfg.EndPoint, s3Cfg.Bucket)
	assert.NoError(t, err)

	ps, err := NewS3PieceStorage(s3Cfg)
	assert.NoError(t, err)

	// gen rand key string
	testkey := "test_key_" + randomString(10)

	_, err = ps.SaveTo(ctx, testkey, strings.NewReader("test"))
	assert.NoError(t, err)

	has, err := ps.Has(ctx, testkey)
	assert.NoError(t, err)
	assert.True(t, has)

	length, err := ps.Len(ctx, testkey)
	assert.NoError(t, err)
	assert.Equal(t, int64(4), length)

	ids, err := ps.ListResourceIds(context.Background())
	assert.NoError(t, err)
	assert.Contains(t, ids, testkey)

	url, err := ps.GetRedirectUrl(ctx, testkey)
	assert.NoError(t, err)
	if s3Cfg.SubDir != "" {
		assert.Contains(t, url, endpoint+"/"+s3Cfg.SubDir+testkey)
	} else {
		assert.Contains(t, url, endpoint+"/"+testkey)
	}

	reader, err := ps.GetReaderCloser(ctx, testkey)
	assert.NoError(t, err)
	defer func() {
		err := reader.Close()
		assert.NoError(t, err)
	}()

	buf := make([]byte, 4)
	n, err := reader.Read(buf)
	assert.ErrorContains(t, err, "EOF")
	assert.Equal(t, 4, n)
	assert.Equal(t, "test", string(buf))

	// remove testkey
	sess := session.Must(session.NewSession(&aws.Config{
		Credentials:      credentials.NewStaticCredentials(s3Cfg.AccessKey, s3Cfg.SecretKey, s3Cfg.Token),
		Endpoint:         aws.String(endpoint),
		S3ForcePathStyle: aws.Bool(false),
		Region:           aws.String(region),
		//LogLevel:         aws.LogLevel(aws.LogDebug),
	}))

	svc := s3.New(sess)
	delKey := testkey
	if s3Cfg.SubDir != "" {
		delKey = s3Cfg.SubDir + testkey

	}
	_, err = svc.DeleteObject(&s3.DeleteObjectInput{
		Bucket: aws.String(s3Cfg.Bucket),
		Key:    aws.String(delKey),
	})
	assert.NoError(t, err)

}

func randomString(n int) string {
	rand.Seed(time.Now().UnixNano())
	var letter = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")

	b := make([]rune, n)
	for i := range b {
		b[i] = letter[rand.Intn(len(letter))]
	}
	return string(b)
}
