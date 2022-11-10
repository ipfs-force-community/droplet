package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/filecoin-project/venus-market/v2/cmd"

	"github.com/urfave/cli/v2"

	"github.com/filecoin-project/go-address"

	"github.com/filecoin-project/venus-market/v2/config"
)

func flagData(cctx *cli.Context, cfg *config.MarketConfig) error {
	if cctx.IsSet(NodeUrlFlag.Name) {
		cfg.Node.Url = cctx.String(NodeUrlFlag.Name)
	}

	if cctx.IsSet(MessagerUrlFlag.Name) {
		cfg.Messager.Url = cctx.String(MessagerUrlFlag.Name)
	}

	if cctx.IsSet(AuthUrlFlag.Name) {
		cfg.AuthNode.Url = cctx.String(AuthUrlFlag.Name)
	}

	signerType := cctx.String(SignerTypeFlag.Name)
	switch signerType {
	case config.SignerTypeGateway:
		{
			if cctx.IsSet(GatewayUrlFlag.Name) {
				cfg.Signer.Url = cctx.String(GatewayUrlFlag.Name)
			}
			if cctx.IsSet(GatewayTokenFlag.Name) {
				cfg.Signer.Token = cctx.String(GatewayTokenFlag.Name)
			}
		}
	case config.SignerTypeWallet:
		{
			if cctx.IsSet(SignerUrlFlag.Name) {
				cfg.Signer.Url = cctx.String(SignerUrlFlag.Name)
			}
			if cctx.IsSet(SignerTokenFlag.Name) {
				cfg.Signer.Token = cctx.String(SignerTokenFlag.Name)
			}
		}
	case config.SignerTypeLotusnode:
		{
			if cctx.IsSet(NodeUrlFlag.Name) {
				cfg.Signer.Url = cctx.String(NodeUrlFlag.Name)
			}
			if cctx.IsSet(NodeTokenFlag.Name) {
				cfg.Signer.Token = cctx.String(NodeTokenFlag.Name)
			}
		}
	default:
		return fmt.Errorf("unsupport signer type %s", signerType)
	}
	cfg.Signer.SignerType = signerType

	if cctx.IsSet(AuthTokeFlag.Name) {
		cfg.Node.Token = cctx.String(AuthTokeFlag.Name)

		if len(cfg.AuthNode.Url) > 0 {
			cfg.AuthNode.Token = cctx.String(AuthTokeFlag.Name)
		}

		if len(cfg.Messager.Url) > 0 {
			cfg.Messager.Token = cctx.String(AuthTokeFlag.Name)
		}

		if len(cfg.Signer.Url) > 0 {
			cfg.Signer.Token = cctx.String(AuthTokeFlag.Name)
		}
	}

	if cctx.IsSet(NodeTokenFlag.Name) {
		cfg.Node.Token = cctx.String(NodeTokenFlag.Name)
	}
	if cctx.IsSet(MessagerTokenFlag.Name) {
		cfg.Messager.Token = cctx.String(MessagerTokenFlag.Name)
	}

	if cctx.IsSet(MysqlDsnFlag.Name) {
		cfg.Mysql.ConnectionString = cctx.String(MysqlDsnFlag.Name)
	}

	if cctx.IsSet(MinerListFlag.Name) {
		storageMiners := make(map[config.Address]struct{}, 0)
		for _, storageMiner := range cfg.StorageMiners {
			if _, ok := storageMiners[storageMiner.Addr]; !ok {
				storageMiners[storageMiner.Addr] = struct{}{}
			}
		}

		addrStrs := cctx.StringSlice(MinerListFlag.Name)
		for _, miners := range addrStrs {
			addrStr := strings.Split(miners, ":")
			addr, err := address.NewFromString(addrStr[0])
			if err != nil {
				return fmt.Errorf("flag provide a wrong address %s %w", addrStr, err)
			}
			account := ""
			if len(addrStr) >= 2 {
				account = addrStr[1]
			}

			if _, ok := storageMiners[config.Address(addr)]; !ok {
				cfg.StorageMiners = append(cfg.StorageMiners, config.User{
					Addr:    config.Address(addr),
					Account: account,
				})
			}
		}
	}

	if cctx.IsSet(RetrievalPaymentAddress.Name) {
		addrStr := cctx.String(RetrievalPaymentAddress.Name)
		addr, err := address.NewFromString(addrStr)
		if err != nil {
			return fmt.Errorf("flag provide a wrong address %s %w", addrStr, err)
		}

		cfg.RetrievalPaymentAddress = config.Address(addr)
	}
	return nil
}

func prepare(cctx *cli.Context, defSignerType config.SignerType) (*config.MarketConfig, error) {
	if !cctx.IsSet(SignerTypeFlag.Name) {
		if err := cctx.Set(SignerTypeFlag.Name, defSignerType); err != nil {
			return nil, fmt.Errorf("set `%s` with wallet failed %w", SignerTypeFlag.Name, err)
		}
	}
	cfg := config.DefaultMarketConfig
	cfg.HomeDir = cctx.String(RepoFlag.Name)
	cfgPath, err := cfg.ConfigPath()
	if err != nil {
		return nil, err
	}

	mainLog.Info("load config from path ", cfgPath)
	if _, err := os.Stat(cfgPath); os.IsNotExist(err) {
		//create
		err = flagData(cctx, cfg)
		if err != nil {
			return nil, fmt.Errorf("parser data from flag %w", err)
		}

		err = config.SaveConfig(cfg)
		if err != nil {
			return nil, fmt.Errorf("save config to %s %w", cfgPath, err)
		}
	} else if err == nil {
		//loadConfig
		err = config.LoadConfig(cfgPath, cfg)
		if err != nil {
			return nil, err
		}

		err = flagData(cctx, cfg)
		if err != nil {
			return nil, fmt.Errorf("parser data from flag %w", err)
		}
	} else {
		return nil, err
	}

	return cfg, cmd.FetchAndLoadBundles(cctx.Context, cfg.Node)
}
