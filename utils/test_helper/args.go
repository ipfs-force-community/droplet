package test_helper

import (
	"flag"
	"testing"

	"github.com/filecoin-project/venus/pkg/testhelpers"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/venus/venus-shared/types"
)

// use "go test ..." with suffix '-local=true' to set 'local' args
var localTest = flag.Bool("local", false, "Run local go tests")

// use "go test ... -mysql="root:ko2005@tcp(127.0.0.1:3306)/storage_market?charset=utf8mb4&parseTime=True&loc=Local&timeout=10s"
var mysqlConn = flag.String("mysql", "", "connection string of testing mysql database")

func Mysql(t *testing.T) string {
	if len(*mysqlConn) == 0 {
		t.Skipf("skip %s, without -mysql args", t.Name())
	}
	return *mysqlConn
}

func LocalTest(t *testing.T) {
	if !*localTest {
		t.Skipf("skip %s, not a local test", t.Name())
	}
}

func MakeTestBlock(t *testing.T) *types.BlockHeader {
	addrGetter := address.NewForTestGetter()
	minerAddr := addrGetter()
	mockCid := testhelpers.CidFromString(t, "mock")
	c1 := testhelpers.CidFromString(t, "a")
	s1 := testhelpers.CidFromString(t, "state1")
	var h1 abi.ChainEpoch = 1
	return &types.BlockHeader{Miner: minerAddr, Messages: mockCid, ParentMessageReceipts: mockCid, Parents: types.NewTipSetKey(c1).Cids(), ParentStateRoot: s1, Height: h1}
}

func MakeTestTipset(t *testing.T) *types.TipSet {
	return testhelpers.RequireNewTipSet(t, MakeTestBlock(t))
}
