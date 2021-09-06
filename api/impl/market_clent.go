package impl

import "github.com/filecoin-project/venus-market/client"

type MarketClientNodeImpl struct {
	client.API
	FundAPI
}
