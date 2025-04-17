package dealfilter

import (
	"bytes"
	"context"
	"encoding/json"
	"os/exec"
	"time"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"

	types "github.com/filecoin-project/venus/venus-shared/types/market"
	"github.com/ipfs-force-community/droplet/v2/config"
	types2 "github.com/ipfs-force-community/droplet/v2/types"
)

const agent = "boost"
const jsonVersion = "2.2.0"

func CliStorageDealFilter(cfg *config.MarketConfig) config.StorageDealFilter {
	return func(ctx context.Context, mAddr address.Address, dealParams *types2.DealParams) (bool, string, error) {
		pCfg, err := cfg.MinerProviderConfig(mAddr, true)
		if err != nil {
			return false, "", err
		}
		if pCfg == nil || len(pCfg.Filter) == 0 {
			return true, "", nil
		}

		type worker struct {
			ID     string
			Start  time.Time
			Stage  string
			Sector int32
		}

		type SealingPipelineState struct {
			SectorStates map[string]int
			Workers      []*worker
		}

		type StorageState struct {
			// The total number of bytes allocated for incoming data
			TotalAvailable uint64
			// The number of bytes reserved for accepted deals
			Tagged uint64
			// The number of bytes that have been downloaded and are waiting to be added to a sector
			Staged uint64
			// The number of bytes that are not tagged
			Free uint64
		}

		type SMAEscrow struct {
			// Funds tagged for ongoing deals
			Tagged abi.TokenAmount
			// Funds in escrow available to be used for deal making
			Available abi.TokenAmount
			// Funds in escrow that are locked for ongoing deals
			Locked abi.TokenAmount
		}

		type CollatWallet struct {
			// The wallet address
			Address string
			// The wallet balance
			Balance abi.TokenAmount
		}

		type PubMsgWallet struct {
			// The wallet address
			Address string
			// The wallet balance
			Balance abi.TokenAmount
			// The funds that are tagged for ongoing deals
			Tagged abi.TokenAmount
		}

		type FundsState struct {
			// Funds in the Storage Market Actor
			Escrow SMAEscrow
			// Funds in the wallet used for deal collateral
			Collateral CollatWallet
			// Funds in the wallet used to pay for Publish Storage Deals messages
			PubMsg PubMsgWallet
		}

		sealingPipelineState := SealingPipelineState{}
		storageState := StorageState{}
		fundsState := FundsState{}

		d := struct {
			*types2.DealParams
			SealingPipelineState *SealingPipelineState
			FundsState           *FundsState
			StorageState         *StorageState
			DealType             string
			FormatVersion        string
			Agent                string
		}{
			DealParams:           dealParams,
			SealingPipelineState: &sealingPipelineState,
			FundsState:           &fundsState,
			StorageState:         &storageState,
			DealType:             "storage",
			FormatVersion:        jsonVersion,
			Agent:                agent,
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
