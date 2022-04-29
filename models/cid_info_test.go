package models

import (
	"context"
	"testing"

	"github.com/filecoin-project/go-fil-markets/piecestore"
	"github.com/filecoin-project/venus-market/v2/models/badger"
	"github.com/filecoin-project/venus-market/v2/models/repo"
	"github.com/ipfs/go-cid"
	"github.com/stretchr/testify/require"

	"github.com/stretchr/testify/assert"
)

func TestCIDInfo(t *testing.T) {
	t.Run("badger", func(t *testing.T) {
		db := BadgerDB(t)
		doTestCidinfo(t, badger.NewBadgerCidInfoRepo(db))
	})

	t.Run("mysql", func(t *testing.T) {
		repo := MysqlDB(t)
		cidInfoRepo := repo.CidInfoRepo()
		defer func() { require.NoError(t, repo.Close()) }()
		doTestCidinfo(t, cidInfoRepo)
	})
}

func doTestCidinfo(t *testing.T, repo repo.ICidInfoRepo) {
	pieceCid := randCid(t)
	payLoadCid1 := randCid(t)
	payLoadCid2 := randCid(t)

	ctx := context.Background()
	blkLocations := map[cid.Cid]piecestore.BlockLocation{
		payLoadCid1: {BlockSize: 10, RelOffset: 999},
		payLoadCid2: {BlockSize: 30, RelOffset: 25},
	}
	err := repo.AddPieceBlockLocations(ctx, pieceCid, blkLocations)
	assert.NoError(t, err)

	cidInfo, err := repo.GetCIDInfo(ctx, payLoadCid1)
	assert.NoError(t, err)

	require.Equal(t, len(cidInfo.PieceBlockLocations), 1)
	require.Equal(t, cidInfo.CID, payLoadCid1)
	require.Equal(t, cidInfo.PieceBlockLocations[0].PieceCID, pieceCid)
	require.Equal(t, cidInfo.PieceBlockLocations[0].BlockSize, uint64(10))
	require.Equal(t, cidInfo.PieceBlockLocations[0].RelOffset, uint64(999))

	cids, err := repo.ListCidInfoKeys(ctx)

	require.NoError(t, err)
	require.LessOrEqual(t, 2, len(cids))
}
