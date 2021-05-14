package types

import (
	"github.com/filecoin-project/venus/pkg/specactors/builtin/market"
	"time"
)

// PendingDealInfo has info about pending deals and when they are due to be
// published
type PendingDealInfo struct {
	Deals              []market.ClientDealProposal
	PublishPeriodStart time.Time
	PublishPeriod      time.Duration
}
