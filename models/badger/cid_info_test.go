package badger

import (
	"context"
	"testing"

	"github.com/filecoin-project/go-fil-markets/piecestore"
	"github.com/filecoin-project/venus/venus-shared/testutil"
	"github.com/ipfs-force-community/droplet/v2/models/repo"
	"github.com/ipfs/go-cid"
	"github.com/stretchr/testify/assert"
)

func TestAddPieceBlockLocations(t *testing.T) {
	ctx, r, cidInfoCases := prepareCidInfoTest(t)

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
}

func TestGetCIDInfo(t *testing.T) {
	ctx, r, cidInfoCases := prepareCidInfoTest(t)

	inSertCidInfo(ctx, t, r, cidInfoCases[0])
	res, err := r.GetCIDInfo(ctx, cidInfoCases[0].CID)
	assert.NoError(t, err)
	assert.Equal(t, cidInfoCases[0], res)
}

func TestListCidInfoKeys(t *testing.T) {
	ctx, r, cidInfoCases := prepareCidInfoTest(t)

	inSertCidInfo(ctx, t, r, cidInfoCases...)

	cidInfos, err := r.ListCidInfoKeys(ctx)
	assert.NoError(t, err)
	assert.Equal(t, len(cidInfoCases), len(cidInfos))
	for _, info := range cidInfoCases {
		assert.Contains(t, cidInfos, info.CID)
	}
}

func prepareCidInfoTest(t *testing.T) (context.Context, repo.ICidInfoRepo, []piecestore.CIDInfo) {
	repo := setup(t)
	r := repo.CidInfoRepo()

	cidInfoCases := make([]piecestore.CIDInfo, 10)
	testutil.Provide(t, &cidInfoCases)

	return context.Background(), r, cidInfoCases
}

func inSertCidInfo(ctx context.Context, t *testing.T, r repo.ICidInfoRepo, cidInfoCases ...piecestore.CIDInfo) {
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
}
