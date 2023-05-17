package dealfilter

import (
	"bytes"
	"context"
	"encoding/json"
	"os/exec"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-fil-markets/storagemarket"

	"github.com/filecoin-project/venus-market/v2/config"
	vsTypes "github.com/filecoin-project/venus/venus-shared/types"
	types "github.com/filecoin-project/venus/venus-shared/types/market"
)

func CliStorageDealFilter(cfg *config.MarketConfig) config.StorageDealFilter {
	return func(ctx context.Context, mAddr address.Address, deal *types.MinerDeal) (bool, string, error) {
		pCfg, err := cfg.MinerProviderConfig(mAddr, true)
		if err != nil {
			return false, "", err
		}
		if pCfg == nil || len(pCfg.Filter) == 0 {
			return true, "", nil
		}

		isOffline := false
		if deal.Ref != nil && deal.Ref.TransferType == storagemarket.TTManual {
			isOffline = true
		}

		d := struct {
			FormatVersion      string
			IsOffline          bool
			ClientDealProposal vsTypes.ClientDealProposal
			FastRetrieval      bool
			TransferType       string
			DealType           string
			Agent              string
		}{
			IsOffline:          isOffline,
			ClientDealProposal: deal.ClientDealProposal,
			DealType:           "storage",
			Agent:              "venus-market",
			FormatVersion:      "1.0.0",
			FastRetrieval:      deal.FastRetrieval,
			TransferType:       deal.Ref.TransferType,
		}
		return runDealFilter(ctx, pCfg.Filter, d)
	}
}

func CliRetrievalDealFilter(cfg *config.MarketConfig) config.RetrievalDealFilter {
	return func(ctx context.Context, mAddr address.Address, deal types.ProviderDealState) (bool, string, error) {
		pCfg, err := cfg.MinerProviderConfig(mAddr, true)
		if err != nil {
			return false, "", err
		}
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
