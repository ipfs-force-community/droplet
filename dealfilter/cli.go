package dealfilter

import (
	"bytes"
	"context"
	"encoding/json"
	"os/exec"

	"github.com/filecoin-project/go-address"

	"github.com/filecoin-project/venus-market/v2/config"
	types "github.com/filecoin-project/venus/venus-shared/types/market"
)

func CliStorageDealFilter(cfg *config.MarketConfig) config.StorageDealFilter {
	return func(ctx context.Context, mAddr address.Address, deal *types.MinerDeal) (bool, string, error) {
		pCfg := cfg.MinerProviderConfig(mAddr, true)
		if pCfg == nil || len(pCfg.Filter) == 0 {
			return true, "", nil
		}

		d := struct {
			*types.MinerDeal
			DealType string
		}{
			MinerDeal: deal,
			DealType:  "piecestorage",
		}
		return runDealFilter(ctx, pCfg.Filter, d)
	}
}

func CliRetrievalDealFilter(cfg *config.MarketConfig) config.RetrievalDealFilter {
	return func(ctx context.Context, mAddr address.Address, deal types.ProviderDealState) (bool, string, error) {
		pCfg := cfg.MinerProviderConfig(mAddr, true)
		if pCfg == nil || len(pCfg.Filter) == 0 {
			return true, "", nil
		}

		d := struct {
			types.ProviderDealState
			DealType string
		}{
			ProviderDealState: deal,
			DealType:          "retrieval",
		}
		return runDealFilter(ctx, pCfg.RetrievalFilter, d)
	}
}

func runDealFilter(ctx context.Context, cmd string, deal interface{}) (bool, string, error) {
	j, err := json.MarshalIndent(deal, "", "  ")
	if err != nil {
		return false, "", err
	}

	var out bytes.Buffer

	c := exec.Command("sh", "-c", cmd)
	c.Stdin = bytes.NewReader(j)
	c.Stdout = &out
	c.Stderr = &out

	switch err := c.Run().(type) {
	case nil:
		return true, "", nil
	case *exec.ExitError:
		return false, out.String(), nil
	default:
		return false, "filter cmd run error", err
	}
}
