package storageprovider

import (
	"context"
	"fmt"

	"github.com/filecoin-project/go-address"
	cborutil "github.com/filecoin-project/go-cbor-util"
	"github.com/filecoin-project/go-fil-markets/storagemarket"
	"github.com/filecoin-project/go-fil-markets/storagemarket/impl/storedask"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/venus-market/v2/models/repo"
	v1api "github.com/filecoin-project/venus/venus-shared/api/chain/v1"
	"github.com/filecoin-project/venus/venus-shared/types"
	types2 "github.com/filecoin-project/venus/venus-shared/types/market"
	"github.com/ipfs-force-community/metrics"
)

type IStorageAsk interface {
	ListAsk(ctx context.Context) ([]*types2.SignedStorageAsk, error)
	GetAsk(mctx context.Context, Addr address.Address) (*types2.SignedStorageAsk, error)
	SetAsk(ctx context.Context, mAddr address.Address, price abi.TokenAmount, verifiedPrice abi.TokenAmount, duration abi.ChainEpoch, options ...storagemarket.StorageAskOption) error
}

func NewStorageAsk(
	ctx metrics.MetricsCtx,
	repo repo.Repo,
	fullnode v1api.FullNode,
) (IStorageAsk, error) {
	return &StorageAsk{repo: repo.StorageAskRepo(), fullNode: fullnode}, nil
}

type StorageAsk struct {
	repo     repo.IStorageAskRepo
	fullNode v1api.FullNode
}

func (storageAsk *StorageAsk) ListAsk(ctx context.Context) ([]*types2.SignedStorageAsk, error) {
	return storageAsk.repo.ListAsk(ctx)
}

func (storageAsk *StorageAsk) GetAsk(ctx context.Context, miner address.Address) (*types2.SignedStorageAsk, error) {
	return storageAsk.repo.GetAsk(ctx, miner)
}

func (storageAsk *StorageAsk) SetAsk(ctx context.Context, miner address.Address, price abi.TokenAmount, verifiedPrice abi.TokenAmount, duration abi.ChainEpoch, options ...storagemarket.StorageAskOption) error {
	minPieceSize := storedask.DefaultMinPieceSize
	maxPieceSize := storedask.DefaultMaxPieceSize

	var seqno uint64

	if s, _ := storageAsk.GetAsk(ctx, miner); s != nil {
		seqno = s.Ask.SeqNo
		minPieceSize = s.Ask.MinPieceSize
		maxPieceSize = s.Ask.MaxPieceSize
	}

	ts, err := storageAsk.fullNode.ChainHead(ctx)
	if err != nil {
		return fmt.Errorf("problem getting chain head:%w", err)
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

	var signedAsk *types2.SignedStorageAsk

	if signedAsk, err = storageAsk.signAsk(ask); err != nil {
		return fmt.Errorf("miner %s sign data failed: %v", miner.String(), err)
	}

	return storageAsk.repo.SetAsk(ctx, signedAsk)
}

func (storageAsk *StorageAsk) signAsk(ask *storagemarket.StorageAsk) (*types2.SignedStorageAsk, error) {
	askBytes, err := cborutil.Dump(ask)
	if err != nil {
		return nil, err
	}

	// get worker address for miner
	ctx := context.TODO()
	tok, err := storageAsk.fullNode.ChainHead(ctx)
	if err != nil {
		return nil, err
	}

	mi, err := storageAsk.fullNode.StateMinerInfo(ctx, ask.Miner, tok.Key())
	if err != nil {
		return nil, err
	}

	addr, err := storageAsk.fullNode.StateAccountKey(ctx, mi.Worker, tok.Key())
	if err != nil {
		return nil, err
	}

	sig, err := storageAsk.fullNode.WalletSign(ctx, addr, askBytes, types.MsgMeta{
		Type: types.MTStorageAsk,
	})
	if err != nil {
		return nil, err
	}

	return &types2.SignedStorageAsk{Ask: ask, Signature: sig}, nil
}
