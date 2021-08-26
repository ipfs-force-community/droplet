package sealer

import (
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/venus-market/clients"
	"github.com/filecoin-project/venus-market/config"
	"github.com/filecoin-project/venus-market/types"
	"github.com/filecoin-project/venus-market/utils"
)

func MinerAddress(cfg config.MarketConfig) (types.MinerAddress, error) {
	addr, err := address.NewFromString(cfg.MinerAddress)
	if err != nil {
		return types.MinerAddress{}, err
	}
	return types.MinerAddress(addr), nil
}

func NewAddressSelector(cfg *config.MarketConfig) (*AddressSelector, error) {
	return &AddressSelector{
		AddressConfig: cfg.AddressConfig,
	}, nil
}

var SealerOpts = utils.Options(
	//sealer service
	utils.Override(new(clients.IStorageMiner), clients.NewStorageMiner),
	utils.Override(new(types.MinerAddress), MinerAddress), //todo miner single miner todo change to support multiple miner
	utils.Override(new(Unsealer), utils.From(new(clients.IStorageMiner))),
	utils.Override(new(SectorBuilder), utils.From(new(clients.IStorageMiner))),
	utils.Override(new(PieceProvider), NewPieceProvider),
	utils.Override(new(AddressSelector), NewAddressSelector),
)
