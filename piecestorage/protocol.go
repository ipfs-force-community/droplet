package piecestorage

import (
	"fmt"
	"github.com/filecoin-project/venus-market/config"
	"net/url"
	"strings"
)

func ParserProtocol(pro string) (config.PieceStorage, error) {
	fIndex := strings.Index(pro, ":")
	if fIndex == -1 {
		return config.PieceStorage{}, fmt.Errorf("parser piece storage %s", pro)
	}

	protocol := pro[:fIndex]
	dsn := pro[fIndex+1:]
	switch protocol {
	case "fs":
		return config.PieceStorage{
			Fs: config.FsPieceStorage{
				Enable: true,
				Path:   dsn,
			},
		}, nil
	case "s3":
		s3Cfg, err := ParserS3(dsn)
		if err != nil {
			return config.PieceStorage{}, err
		}
		return config.PieceStorage{
			S3: s3Cfg,
		}, nil
	default:
		return config.PieceStorage{}, fmt.Errorf("unsupport protocol %s", protocol)
	}
}

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
