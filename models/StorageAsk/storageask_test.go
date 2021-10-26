package StorageAsk

import (
	"os"
	"os/exec"
	"path"
	"testing"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-fil-markets/storagemarket"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/venus-market/utils/test_helper"
	"github.com/stretchr/testify/require"
)

func testRepo(t *testing.T, repo *StorageAskRepo) {
	miner, _ := address.NewFromString("f02438")
	price := abi.NewTokenAmount(100)
	verifyPrice := abi.NewTokenAmount(10333)
	dur := abi.ChainEpoch(10000)

	ask := &storagemarket.StorageAsk{
		Price:         price,
		VerifiedPrice: verifyPrice,
		Miner:         miner,
	}

	require.NoError(t, repo.SetAsk(miner, ask.Price, ask.VerifiedPrice, dur))

	ask2, err := repo.GetAsk(miner)
	require.NoError(t, err)

	require.Equal(t, ask2.Ask.Miner, miner, "miner should equals : %s", miner.String())
	require.Equal(t, ask2.Ask.Price, price, "price should equals : %s", price.String())

	price = big.Add(price, abi.NewTokenAmount(10000))
	verifyPrice = big.Add(verifyPrice, abi.NewTokenAmount(44))

	ask.Price = price
	ask.VerifiedPrice = verifyPrice

	require.NoError(t, repo.SetAsk(miner, ask.Price, ask.VerifiedPrice, dur))

	ask2, err = repo.GetAsk(miner)
	require.NoError(t, err)

	require.Equal(t, ask2.Ask.Price, price, "price should equals : %s", price.String())
	require.Equal(t, ask2.Ask.VerifiedPrice, verifyPrice, "price should equals : %s", verifyPrice.String())
}

func TestMysqlRepo(t *testing.T) {
	mysql := test_helper.Mysql(t)
	mysqlCfg := &StorageAskCfg{
		DbType: "mysql",
		// "root:ko2005@tcp(127.0.0.1:3306)/storage_market?charset=utf8mb4&parseTime=True&loc=Local&timeout=10s",
		URI:   mysql,
		Debug: true,
	}
	askRepo, err := NewStorageAsk(mysqlCfg, mockProvider{})
	require.NoError(t, err)
	defer askRepo.Close()
	testRepo(t, askRepo)
}

func TestBadgerRepo(t *testing.T) {
	repoPath, _ := os.Getwd()
	repoPath = path.Join(repoPath, "./.badgerStAskDb")

	badgerCfg := &StorageAskCfg{DbType: "badger", URI: repoPath}

	askRepo, err := NewStorageAsk(badgerCfg, mockProvider{})
	require.NoError(t, err)
	defer func() {
		_ = askRepo.Close()
		if err := exec.Command("rm", "-rf", repoPath).Run(); err != nil {
			t.Logf("remove %s failed:%s", repoPath, err.Error())
		}
	}()
	testRepo(t, askRepo)
}
