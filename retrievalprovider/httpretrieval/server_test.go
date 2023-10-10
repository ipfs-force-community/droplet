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

	"github.com/filecoin-project/venus/venus-shared/api/market/v1/mock"
	"github.com/filecoin-project/venus/venus-shared/types"
	"github.com/filecoin-project/venus/venus-shared/types/market"
	"github.com/golang/mock/gomock"
	"github.com/gorilla/mux"
	"github.com/ipfs-force-community/droplet/v2/config"
	"github.com/ipfs-force-community/droplet/v2/piecestorage"
	"github.com/ipfs/go-cid"
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
