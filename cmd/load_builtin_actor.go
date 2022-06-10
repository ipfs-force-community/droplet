package cmd

import (
	"context"
	"fmt"

	"github.com/filecoin-project/venus-market/v2/config"
	v1 "github.com/filecoin-project/venus/venus-shared/api/chain/v1"
	builtinactors "github.com/filecoin-project/venus/venus-shared/builtin-actors"
	"github.com/filecoin-project/venus/venus-shared/types"
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

	networkName, err := fullNodeAPI.StateNetworkName(ctx)
	if err != nil {
		return err
	}

	nt, err := networkNameToNetworkType(networkName)
	if err != nil {
		return err
	}

	return builtinactors.SetNetworkBundle(nt)
}

func networkNameToNetworkType(networkName types.NetworkName) (types.NetworkType, error) {
	switch networkName {
	case "":
		return types.NetworkDefault, fmt.Errorf("network name is empty")
	case "mainnet":
		return types.NetworkMainnet, nil
	case "calibrationnet", "calibnet":
		return types.NetworkCalibnet, nil
	case "butterflynet", "butterfly":
		return types.NetworkButterfly, nil
	case "interopnet", "interop":
		return types.NetworkInterop, nil
	default:
		// include 2k force
		return types.Network2k, nil
	}
}
