package storageadapter

import (
	"context"
	"github.com/filecoin-project/venus-market/metrics"
	"github.com/filecoin-project/venus/app/client/apiface"

	"github.com/filecoin-project/go-address"
	cborutil "github.com/filecoin-project/go-cbor-util"
	"github.com/filecoin-project/go-fil-markets/storagemarket"
	"github.com/filecoin-project/go-fil-markets/storagemarket/impl/storedask"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/venus-market/models/itf"
	"github.com/filecoin-project/venus/pkg/wallet"
	"golang.org/x/xerrors"
)

func NewStorageAsk(ctx metrics.MetricsCtx,
	ds itf.IStorageAskRepo, fullnode apiface.FullNode) (*StorageAsk, error) {
	return &StorageAsk{repo: ds, fullNode: fullnode}, nil
}


type StorageAsk struct {
	repo     itf.IStorageAskRepo
	fullNode apiface.FullNode
}

func (repo *StorageAsk) Close() error {
	return repo.repo.Close()
}

func (repo *StorageAsk) GetAsk(miner address.Address) (*storagemarket.SignedStorageAsk, error) {
	return repo.repo.GetAsk(miner)
}

func (repo *StorageAsk) SetAsk(miner address.Address, price abi.TokenAmount, verifiedPrice abi.TokenAmount, duration abi.ChainEpoch, options ...storagemarket.StorageAskOption) error {
	minPieceSize := storedask.DefaultMinPieceSize
	maxPieceSize := storedask.DefaultMaxPieceSize

	var seqno uint64

	if s, _ := repo.GetAsk(miner); s != nil {
		seqno = s.Ask.SeqNo
		minPieceSize = s.Ask.MinPieceSize
		maxPieceSize = s.Ask.MaxPieceSize
	}

	ctx := context.TODO()

	ts, err := repo.fullNode.ChainHead(ctx)
	if err != nil {
		return xerrors.Errorf("Problem getting chain head:%w", err)
	}

	ask := &storagemarket.StorageAsk{
		Price:         price,
		VerifiedPrice: verifiedPrice,
		Timestamp:     ts.Height(),
		Expiry:        ts.Height() + duration,
		Miner:         miner,
		SeqNo:         seqno,
		MinPieceSize:  minPieceSize,
		MaxPieceSize:  maxPieceSize,
	}

	for _, option := range options {
		option(ask)
	}

	var signedAsk *storagemarket.SignedStorageAsk

	if signedAsk, err = repo.signAsk(ask); err != nil {
		return xerrors.Errorf("Miner:%s sign data failed", miner.String(), err)
	}

	return repo.repo.SetAsk(signedAsk)
}

func (repo *StorageAsk) signAsk(ask *storagemarket.StorageAsk) (*storagemarket.SignedStorageAsk, error) {
	askBytes, err := cborutil.Dump(ask)
	if err != nil {
		return nil, err
	}
	sig, err := repo.fullNode.WalletSign(context.TODO(), ask.Miner, nil, wallet.MsgMeta{
		Type:  wallet.MTStorageAsk,
		Extra: askBytes,
	})
	if err != nil {
		return nil, err
	}
	return &storagemarket.SignedStorageAsk{Ask: ask, Signature: sig}, nil

}
