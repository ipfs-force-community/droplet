package httpretrieval

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"testing"
	"time"

	dagstore2 "github.com/filecoin-project/dagstore"
	"github.com/filecoin-project/go-padreader"
	"github.com/filecoin-project/venus/venus-shared/api/market/v1/mock"
	"github.com/filecoin-project/venus/venus-shared/types"
	"github.com/filecoin-project/venus/venus-shared/types/market"
	"github.com/golang/mock/gomock"
	"github.com/gorilla/mux"
	"github.com/ipfs-force-community/droplet/v2/config"
	"github.com/ipfs-force-community/droplet/v2/dagstore"
	"github.com/ipfs-force-community/droplet/v2/piecestorage"
	"github.com/ipfs/go-cid"
	carindex "github.com/ipld/go-car/v2/index"
	"github.com/multiformats/go-multihash"
	"github.com/stretchr/testify/assert"
)

func TestPathRegexp(t *testing.T) {
	reg, err := regexp.Compile(`/piece/[a-z0-9]+`)
	assert.NoError(t, err)

	cases := []struct {
		str    string
		expect bool
	}{
		{
			str:    "xxx",
			expect: false,
		},
		{
			str:    "/piece/",
			expect: false,
		},
		{
			str:    "/piece/ssss",
			expect: true,
		},
		{
			str:    "/piece/ss1ss1",
			expect: true,
		},
	}

	for _, c := range cases {
		assert.Equal(t, c.expect, reg.MatchString(c.str))
	}
}

func TestRetrievalByPiece(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tmpDri := t.TempDir()
	cfg := config.DefaultMarketConfig
	cfg.Home.HomeDir = tmpDri
	cfg.PieceStorage.Fs = []*config.FsPieceStorage{
		{
			Name:     "test",
			ReadOnly: false,
			Path:     tmpDri,
		},
	}
	assert.NoError(t, config.SaveConfig(cfg))

	pieceStr := "baga6ea4seaqpzcr744w2rvqhkedfqbuqrbo7xtkde2ol6e26khu3wni64nbpaeq"
	piece, err := cid.Decode(pieceStr)
	assert.NoError(t, err)
	buf := &bytes.Buffer{}
	f, err := os.Create(filepath.Join(tmpDri, pieceStr+".car"))
	assert.NoError(t, err)
	assert.NoError(t, os.MkdirAll(filepath.Join(tmpDri, pieceStr), os.ModePerm))
	for i := 0; i < 100; i++ {
		buf.WriteString("TEST TEST\n")
	}
	_, err = f.Write(buf.Bytes())
	assert.NoError(t, err)
	assert.NoError(t, f.Close())

	pieceStorage, err := piecestorage.NewPieceStorageManager(&cfg.PieceStorage)
	assert.NoError(t, err)
	ctrl := gomock.NewController(t)
	m := mock.NewMockIMarket(ctrl)
	m.EXPECT().MarketListIncompleteDeals(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, p *market.StorageDealQueryParams) ([]market.MinerDeal, error) {
			if p.PieceCID != pieceStr {
				return nil, fmt.Errorf("not found deal")
			}
			return append([]market.MinerDeal{}, market.MinerDeal{ClientDealProposal: types.ClientDealProposal{Proposal: types.DealProposal{PieceCID: piece}}}), nil
		}).AnyTimes()

	s, err := NewServer(ctx, pieceStorage, m, nil)
	assert.NoError(t, err)
	port := "34897"
	startHTTPServer(ctx, t, port, s)

	url := fmt.Sprintf("http://127.0.0.1:%s/piece/%s", port, pieceStr)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	assert.NoError(t, err)
	resp, err := http.DefaultClient.Do(req)
	assert.NoError(t, err)
	defer resp.Body.Close() // nolint

	data, err := io.ReadAll(resp.Body)
	assert.NoError(t, err)
	assert.Equal(t, buf.Bytes(), data)

	// deal not exist
	url = fmt.Sprintf("http://127.0.0.1:%s/piece/%s", port, "bafybeiakou6e7hnx4ms2yangplzl6viapsoyo6phlee6bwrg4j2xt37m3q")
	req, err = http.NewRequest(http.MethodGet, url, nil)
	assert.NoError(t, err)
	resp, err = http.DefaultClient.Do(req)
	assert.NoError(t, err)
	defer resp.Body.Close() // nolint
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func startHTTPServer(ctx context.Context, t *testing.T, port string, s *Server) {
	mux := mux.NewRouter()
	err := mux.HandleFunc("/piece/{cid}", s.retrievalByPieceCID).GetError()
	assert.NoError(t, err)
	err = mux.HandleFunc("/ipfs/{cid}", s.retrievalByIPFS).GetError()
	assert.NoError(t, err)

	ser := &http.Server{
		Addr:    "127.0.0.1:" + port,
		Handler: mux,
	}

	go func() {
		if err := ser.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			assert.NoError(t, err)
		}
	}()

	go func() {
		// wait server exit
		<-ctx.Done()
		assert.NoError(t, ser.Shutdown(context.TODO()))
	}()
	// wait serve up
	time.Sleep(time.Second * 2)
}

func TestTrustless(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pieceStr := "baga6ea4seaqa6u2eajfj57t2laudfkdmxmzv4nix255qytfgcr2uoexspketoda"
	piece, err := cid.Decode(pieceStr)
	assert.NoError(t, err)

	ctrl := gomock.NewController(t)
	m := mock.NewMockIMarket(ctrl)
	m.EXPECT().MarketListIncompleteDeals(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, p *market.StorageDealQueryParams) ([]market.MinerDeal, error) {
			if p.PieceCID != pieceStr {
				return nil, fmt.Errorf("not found deal")
			}
			return append([]market.MinerDeal{}, market.MinerDeal{ClientDealProposal: types.ClientDealProposal{Proposal: types.DealProposal{PieceCID: piece}}}), nil
		}).AnyTimes()

	var blocks []cid.Cid
	dagStoreWrapper := dagstore.NewMockDagStoreWrapper()
	indexPath := "./testdata/baga6ea4seaqa6u2eajfj57t2laudfkdmxmzv4nix255qytfgcr2uoexspketoda.full.idx"
	f, err := os.Open(indexPath)
	assert.NoError(t, err)
	idx, err := carindex.ReadFrom(f)
	assert.NoError(t, err)
	err = idx.(carindex.IterableIndex).ForEach(func(mh multihash.Multihash, offset uint64) error {
		blockCid := cid.NewCidV1(cid.Raw, mh)
		blocks = append(blocks, blockCid)
		dagStoreWrapper.AddBlockToPieceIndex(blockCid, piece)

		return nil
	})
	assert.NoError(t, err)
	assert.NoError(t, f.Close())

	piecePath := "./testdata/baga6ea4seaqa6u2eajfj57t2laudfkdmxmzv4nix255qytfgcr2uoexspketoda"
	resch := make(chan dagstore2.ShardResult, 1)
	err = dagStoreWrapper.RegisterShardWithIndex(ctx, piece, piecePath, true, resch, idx)
	assert.NoError(t, err)
	close(resch)

	s, err := NewServer(ctx, nil, m, dagStoreWrapper)
	assert.NoError(t, err)
	port := "34898"
	startHTTPServer(ctx, t, port, s)

	for _, blockCid := range blocks {
		url := fmt.Sprintf("http://127.0.0.1:%s/ipfs/%s", port, blockCid)
		req, err := http.NewRequest(http.MethodGet, url, nil)
		assert.NoError(t, err)
		req.Header.Set("Accept", "application/vnd.ipld.car; version=1; order=dfs; dups=n")
		resp, err := http.DefaultClient.Do(req)
		assert.NoError(t, err)

		data, err := io.ReadAll(resp.Body)
		assert.NoError(t, err)
		fmt.Println("block:", blockCid, "data:", string(data))

		assert.NoError(t, resp.Body.Close())
	}
}

func TestRetrievalPaddingPiece(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tmpDri := t.TempDir()
	cfg := config.DefaultMarketConfig
	cfg.Home.HomeDir = tmpDri
	cfg.PieceStorage.Fs = []*config.FsPieceStorage{
		{
			Name:     "test",
			ReadOnly: false,
			Path:     tmpDri,
		},
	}
	assert.NoError(t, config.SaveConfig(cfg))

	pieceStr := "baga6ea4seaqpzcr744w2rvqhkedfqbuqrbo7xtkde2ol6e26khu3wni64nbpaeq"
	piece, err := cid.Decode(pieceStr)
	assert.NoError(t, err)
	buf := &bytes.Buffer{}
	f, err := os.Create(filepath.Join(tmpDri, pieceStr))
	assert.NoError(t, err)
	for i := 0; i < 100; i++ {
		buf.WriteString("TEST TEST\n")
	}
	_, err = f.Write(buf.Bytes())
	assert.NoError(t, err)
	assert.NoError(t, f.Close())

	pieceStorage, err := piecestorage.NewPieceStorageManager(&cfg.PieceStorage)
	assert.NoError(t, err)
	ctrl := gomock.NewController(t)
	m := mock.NewMockIMarket(ctrl)
	m.EXPECT().MarketListIncompleteDeals(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, p *market.StorageDealQueryParams) ([]market.MinerDeal, error) {
			if p.PieceCID != pieceStr {
				return nil, fmt.Errorf("not found deal")
			}
			return append([]market.MinerDeal{}, market.MinerDeal{ClientDealProposal: types.ClientDealProposal{Proposal: types.DealProposal{PieceCID: piece}}}), nil
		}).AnyTimes()

	s, err := NewServer(ctx, pieceStorage, m, nil)
	assert.NoError(t, err)
	port := "34897"
	startHTTPServer(ctx, t, port, s)

	carSize := len(buf.Bytes())
	paddedSize := padreader.PaddedSize(uint64(carSize))

	cases := []struct {
		r      string
		expect []byte
	}{
		{
			r:      fmt.Sprintf("%d-%d", 0, 99),
			expect: buf.Bytes()[0:100],
		},
		{
			r:      fmt.Sprintf("%d-%d", 0, carSize-1),
			expect: buf.Bytes(),
		},
		{
			r:      fmt.Sprintf("%d-%d", 0, carSize+10),
			expect: append(buf.Bytes(), make([]byte, 11)...),
		},
		{
			r:      "0-",
			expect: append(buf.Bytes(), make([]byte, int(paddedSize)-carSize)...),
		},
	}

	for _, c := range cases {
		url := fmt.Sprintf("http://127.0.0.1:%s/piece/%s", port, pieceStr)
		req, err := http.NewRequest(http.MethodGet, url, nil)
		assert.NoError(t, err)
		req.Header.Set("Range", fmt.Sprintf("bytes=%s", c.r))
		resp, err := http.DefaultClient.Do(req)
		assert.NoError(t, err)
		defer resp.Body.Close() // nolint

		data, err := io.ReadAll(resp.Body)
		assert.NoError(t, err)
		assert.Equal(t, c.expect, data)
	}
}
