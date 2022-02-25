package piecestorage

import (
	"github.com/filecoin-project/venus-market/config"
	"github.com/ipfs-force-community/venus-common-utils/builder"
	"golang.org/x/xerrors"
	"reflect"
	"strings"
)

var PieceStorageOpts = func(cfg *config.MarketConfig) builder.Option {
	return builder.Options(
		//piece
		builder.Override(new(IPieceStorage), func(cfg *config.PieceStorage) (IPieceStorage, error) {
			return NewPieceStorage(cfg)
		}), //save read piece data
	)
}

func NewPieceStorage(cfg interface{}) (IPieceStorage, error) {
	//todo only use one storage current
	multiEnable := 0
	var storage IPieceStorage
	var err error

	val := reflect.Indirect(reflect.ValueOf(cfg))
	cfgT := val.Type()
	for i := 0; i < val.NumField(); i++ {
		if val.Field(i).FieldByName("Enable").Bool() {
			multiEnable++
			protocol := strings.ToLower(cfgT.Field(i).Name)
			resolver, err := GetPieceProtocolResolve(Protocol(protocol))
			if err != nil {
				return nil, err
			}
			storage, err = resolver.Constructor(val.Field(i).Interface())
			if err != nil {
				return nil, err
			}
		}
	}
	if multiEnable == 0 {
		return nil, xerrors.New("must config a piece storage ")
	} else if multiEnable > 1 {
		return nil, xerrors.New("can only config one piece storage")
	}
	return storage, err
}
