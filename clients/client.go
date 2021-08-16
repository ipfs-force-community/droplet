package clients

import (
	"context"
	"github.com/filecoin-project/go-jsonrpc"
	"github.com/filecoin-project/lotus/api"
	"github.com/filecoin-project/venus-market/config"
	"github.com/ipfs-force-community/venus-common-utils/apiinfo"
)

func NewNodeClient(ctx context.Context, cfg *config.Node) (api.FullNode, jsonrpc.ClientCloser, error) {
	fullNode := api.FullNodeStruct{}

	aInfo := apiinfo.NewAPIInfo(cfg.Url, cfg.Token)
	addr, err := aInfo.DialArgs("v1")
	if err != nil {
		return nil, nil, err
	}
	closer, err := jsonrpc.NewMergeClient(ctx, addr, "Filecoin", api.GetInternalStructs(fullNode), aInfo.AuthHeader())

	return &fullNode, closer, err
}
