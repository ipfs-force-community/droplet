package piecestorage

import (
	"fmt"
	"github.com/filecoin-project/venus-market/config"
	"golang.org/x/xerrors"
	"strings"
	"sync"
)

type Protocol string

type ProtocolResolver func(cfg string) (config.PieceStorage, error)

const (
	FS Protocol = "fs"
	S3 Protocol = "s3"
)

var protocolRegistry map[Protocol]ProtocolResolver
var lk sync.Mutex

func init() {
	protocolRegistry = map[Protocol]ProtocolResolver{}
	protocolRegistry[FS] = func(cfg string) (config.PieceStorage, error) {
		return config.PieceStorage{
			Fs: config.FsPieceStorage{
				Enable: true,
				Path:   cfg,
			},
		}, nil
	}
	protocolRegistry[S3] = func(cfg string) (config.PieceStorage, error) {
		s3Cfg, err := ParserS3(cfg)
		if err != nil {
			return config.PieceStorage{}, err
		}
		return config.PieceStorage{
			S3: s3Cfg,
		}, nil
	}
}

func RegisterPieceStorage(protocol Protocol, resolver ProtocolResolver) {
	lk.Lock()
	defer lk.Unlock()
	protocolRegistry[protocol] = resolver
}

func GetPieceProtocolResolve(protocol Protocol) (ProtocolResolver, error) {
	lk.Lock()
	defer lk.Unlock()
	resolver, ok := protocolRegistry[protocol]
	if !ok {
		return nil, xerrors.Errorf("unable to find resolver for protocol %s", protocol)
	}
	return resolver, nil
}

func ParserProtocol(pro string) (config.PieceStorage, error) {
	fIndex := strings.Index(pro, ":")
	if fIndex == -1 {
		return config.PieceStorage{}, fmt.Errorf("parser piece storage %s", pro)
	}

	protocol := pro[:fIndex]
	dsn := pro[fIndex+1:]

	resolver, err := GetPieceProtocolResolve(Protocol(protocol))
	if err != nil {
		return config.PieceStorage{}, err
	}
	return resolver(dsn)
}
