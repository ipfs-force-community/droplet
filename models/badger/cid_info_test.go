package badger

import (
	"context"
	"testing"

	"github.com/filecoin-project/go-fil-markets/piecestore"
	"github.com/filecoin-project/venus/venus-shared/testutil"
	"github.com/ipfs/go-cid"
	"github.com/stretchr/testify/assert"
)

func TestCidInfo(t *testing.T) {
	ctx := context.Background()
	repo := setup(t)
	r := repo.CidInfoRepo()

	cidInfoCases := make([]piecestore.CIDInfo, 10)
	testutil.Provide(t, &cidInfoCases)

	t.Run("AddPieceBlockLocations", func(t *testing.T) {

		pieceCid2cidInfo := make(map[cid.Cid][]piecestore.CIDInfo)
		for _, info := range cidInfoCases {
			for _, location := range info.PieceBlockLocations {
				if _, ok := pieceCid2cidInfo[location.PieceCID]; !ok {
					pieceCid2cidInfo[location.PieceCID] = make([]piecestore.CIDInfo, 0)
				}
				pieceCid2cidInfo[location.PieceCID] = append(pieceCid2cidInfo[location.PieceCID], info)
			}
		}

		for pieceCid, cidInfo := range pieceCid2cidInfo {
			playloadCid2location := make(map[cid.Cid]piecestore.BlockLocation)
			for _, info := range cidInfo {
				for _, location := range info.PieceBlockLocations {
					playloadCid2location[info.CID] = location.BlockLocation
				}
			}
			err := r.AddPieceBlockLocations(ctx, pieceCid, playloadCid2location)
			assert.NoError(t, err)
		}
	})

	t.Run("GetCIDInfo", func(t *testing.T) {
		res, err := r.GetCIDInfo(ctx, cidInfoCases[0].CID)
		assert.NoError(t, err)
		assert.Equal(t, cidInfoCases[0], res)
	})

	t.Run("ListCidInfoKeys", func(t *testing.T) {
		cidInfos, err := r.ListCidInfoKeys(ctx)
		assert.NoError(t, err)
		assert.Equal(t, len(cidInfoCases), len(cidInfos))
		for _, info := range cidInfoCases {
			assert.Contains(t, cidInfos, info.CID)
		}
	})
}
