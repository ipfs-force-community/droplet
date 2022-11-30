package retrievalprovider

import (
	"time"

	"github.com/ipfs-force-community/metrics"

	"github.com/filecoin-project/venus-auth/jwtclient"

	"github.com/ipfs-force-community/venus-gateway/marketevent"
	"github.com/ipfs-force-community/venus-gateway/types"
	"github.com/ipfs-force-community/venus-gateway/validator"

	gatewayAPIV2 "github.com/filecoin-project/venus/venus-shared/api/gateway/v2"
)

func NewMarketEventStream(mCtx metrics.MetricsCtx, authClient *jwtclient.AuthClient) gatewayAPIV2.IMarketEvent {

	marketStream := marketevent.NewMarketEventStream(mCtx, validator.NewMinerValidator(authClient), &types.RequestConfig{
		RequestQueueSize: 30,
		RequestTimeout:   time.Hour * 7, // wait seven hour to do unseal
		ClearInterval:    time.Minute * 5,
	})

	return marketStream
}
