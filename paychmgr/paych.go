package paychmgr

import (
	clients2 "github.com/filecoin-project/venus-market/api/clients"
	"github.com/filecoin-project/venus-market/metrics"
	"github.com/filecoin-project/venus-market/models"
	"github.com/filecoin-project/venus/app/client/apiface"
	"github.com/filecoin-project/venus/pkg/paychmgr"
)

func NewManager(ctx metrics.MetricsCtx, ds models.PayChanDS, messager clients2.IMessager, walletClient clients2.ISinger, fullNode apiface.FullNode) (*paychmgr.Manager, error) {
	//todo  to use really messager?
	return paychmgr.NewManager(ctx, ds, &paychmgr.ManagerParams{
		MPoolAPI:     fullNode,
		ChainInfoAPI: fullNode,
		WalletAPI:    walletClient,
		SM:           NewStateMgrAdapter(fullNode),
	})
}
