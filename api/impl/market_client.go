package impl

import (
	"context"

	"github.com/ipfs/go-cid"

	"github.com/ipfs-force-community/droplet/v2/api/clients"
	"github.com/ipfs-force-community/droplet/v2/client"
	"github.com/ipfs-force-community/droplet/v2/version"

	"github.com/filecoin-project/venus/pkg/constants"
	clientapi "github.com/filecoin-project/venus/venus-shared/api/market/client"
	vTypes "github.com/filecoin-project/venus/venus-shared/types"
)

var _ clientapi.IMarketClient = (*MarketClientNodeImpl)(nil)

type MarketClientNodeImpl struct {
	client.API
	FundAPI
	Messager clients.IMixMessage
}

func (m *MarketClientNodeImpl) MessagerWaitMessage(ctx context.Context, mid cid.Cid) (*vTypes.MsgLookup, error) {
	// WaitMsg method has been replace in messager mode
	return m.Messager.WaitMsg(ctx, mid, constants.MessageConfidence, constants.LookbackNoLimit, false)
}

func (m *MarketClientNodeImpl) MessagerPushMessage(ctx context.Context, msg *vTypes.Message, meta *vTypes.MessageSendSpec) (cid.Cid, error) {
	var spec *vTypes.MessageSendSpec
	if meta != nil {
		spec = &vTypes.MessageSendSpec{
			MaxFee:            meta.MaxFee,
			GasOverEstimation: meta.GasOverEstimation,
		}
	}

	return m.Messager.PushMessage(ctx, msg, spec)
}

func (m *MarketClientNodeImpl) MessagerGetMessage(ctx context.Context, mid cid.Cid) (*vTypes.Message, error) {
	// ChainGetMessage method has been replace in messager mode
	return m.Messager.GetMessage(ctx, mid)
}

func (m *MarketClientNodeImpl) Version(ctx context.Context) (vTypes.Version, error) {
	return vTypes.Version{Version: version.UserVersion()}, nil
}
