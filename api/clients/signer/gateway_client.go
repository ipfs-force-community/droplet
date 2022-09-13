package signer

import (
	"github.com/filecoin-project/go-jsonrpc"

	"github.com/ipfs-force-community/metrics"

	"github.com/filecoin-project/venus-market/v2/config"

	"github.com/filecoin-project/venus/venus-shared/api"
	v2GatewayAPI "github.com/filecoin-project/venus/venus-shared/api/gateway/v2"
)

func newGatewayWalletClient(mCtx metrics.MetricsCtx, nodeCfg *config.Signer) (ISigner, jsonrpc.ClientCloser, error) {
	info := api.NewAPIInfo(nodeCfg.Url, nodeCfg.Token)
	dialAddr, err := info.DialArgs("v2")
	if err != nil {
		return nil, nil, err
	}

	gatewayAPI, closer, err := v2GatewayAPI.NewIGatewayRPC(mCtx, dialAddr, info.AuthHeader())
	if err != nil {
		return nil, nil, err
	}

	return gatewayAPI, closer, nil
}
