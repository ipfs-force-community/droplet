package paychmgr

import (
	"context"
	"github.com/filecoin-project/venus-market/clients"
	"github.com/filecoin-project/venus-market/models"
	"github.com/filecoin-project/venus/app/client/apiface"
	"github.com/filecoin-project/venus/pkg/paychmgr"
)

func NewManager(ctx context.Context, ds models.PayChanDS, messager clients.IMessager, walletClient clients.IWalletClient, fullNode apiface.FullNode) *paychmgr.Manager {
	return paychmgr.NewManager(ctx, ds, &paychmgr.ManagerParams{
		MPoolAPI:     &MessagePullAdapter{},
		ChainInfoAPI: fullNode,
		WalletAPI:    walletClient,
		SM:           &StateMgrAdapter{},
	})
}
