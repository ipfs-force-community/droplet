package storageprovider

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDealMetric(t *testing.T) {
	t.SkipNow()
	ctx := context.Background()
	miner := "f02002200"

	c, err := getMinerEligibleDealCount(ctx, miner)
	assert.NoError(t, err)
	fmt.Println("count:", c)

	rate, err := getMinerRetrievalRate(ctx, miner)
	assert.NoError(t, err)
	fmt.Println("rate:", rate)
}
