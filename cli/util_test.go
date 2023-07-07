package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestUnmarshalBoostDeal(t *testing.T) {
	data, err := os.ReadFile("./testdata/boost_deal_query_result.json")
	assert.NoError(t, err)

	var r result
	assert.NoError(t, json.Unmarshal(data, &r))
	assert.Equal(t, 2, r.BoostResult.Deals.TotalCount)
	assert.Len(t, r.BoostResult.Deals.Deals, 2)
	fmt.Printf("%+v\n", r.BoostResult.Deals)

	deal := r.BoostResult.Deals.Deals[0]
	assert.Equal(t, "Awaiting Offline Data Import", deal.Message)
	createAt, err := time.Parse(time.RFC3339, deal.CreatedAt)
	assert.NoError(t, err)
	fmt.Println("create at", createAt)

	data, err = os.ReadFile("./testdata/lotus_miner_query_result.json")
	assert.NoError(t, err)

	var r2 result
	assert.NoError(t, json.Unmarshal(data, &r2))
	assert.Len(t, r2.Result, 3)
	assert.Equal(t, uint64(7), r2.Result[0].State)
	for i, deal := range r2.Result {
		fmt.Printf("i: %d, deal: %+v\n", i, deal)
	}
}
