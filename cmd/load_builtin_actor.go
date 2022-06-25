package cmd

import (
	"context"

	"github.com/filecoin-project/venus-market/v2/config"
	v1 "github.com/filecoin-project/venus/venus-shared/api/chain/v1"
	"github.com/filecoin-project/venus/venus-shared/utils"
	"github.com/ipfs-force-community/venus-common-utils/apiinfo"
)

func FetchAndLoadBundles(ctx context.Context, nodeCfg config.Node) error {
	apiInfo := apiinfo.NewAPIInfo(nodeCfg.Url, nodeCfg.Token)
	addr, err := apiInfo.DialArgs("v1")
	if err != nil {
		return err
	}
	fullNodeAPI, closer, err := v1.NewFullNodeRPC(ctx, addr, apiInfo.AuthHeader())
	if err != nil {
		return err
	}
	defer closer()

	return utils.LoadBuiltinActors(ctx, fullNodeAPI)
}
