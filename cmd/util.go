package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/ipfs-force-community/droplet/v2/config"
	"github.com/mitchellh/go-homedir"
	"github.com/urfave/cli/v2"

	"github.com/filecoin-project/venus/venus-shared/api"
	v1 "github.com/filecoin-project/venus/venus-shared/api/chain/v1"
	"github.com/filecoin-project/venus/venus-shared/utils"
)

func FetchAndLoadBundles(ctx context.Context, nodeCfg config.Node) error {
	apiInfo := api.NewAPIInfo(nodeCfg.Url, nodeCfg.Token)
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

func GetRepoPath(cctx *cli.Context, repoFlagName, oldRepoPath string) (string, error) {
	repoPath, err := homedir.Expand(cctx.String(repoFlagName))
	if err != nil {
		return "", err
	}
	has, err := exist(repoPath)
	if err != nil {
		return "", err
	}
	if !has {
		oldRepoPath, err = homedir.Expand(oldRepoPath)
		if err != nil {
			return "", err
		}
		has, err = exist(oldRepoPath)
		if err != nil {
			return "", err
		}
		if has {
			return oldRepoPath, nil
		}
	}

	return repoPath, nil
}

func exist(path string) (bool, error) {
	f, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	if !f.IsDir() {
		return false, fmt.Errorf("%s not a file directory", path)
	}

	return true, nil
}
