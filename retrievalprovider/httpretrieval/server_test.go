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
	"sync"
	"testing"
	"time"

	"github.com/filecoin-project/venus-market/v2/config"
	"github.com/gorilla/mux"
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
	buf := &bytes.Buffer{}
	f, err := os.Create(filepath.Join(tmpDri, pieceStr))
	assert.NoError(t, err)
	for i := 0; i < 100; i++ {
		buf.WriteString("TEST TEST\n")
	}
	_, err = f.Write(buf.Bytes())
	assert.NoError(t, err)
	assert.NoError(t, f.Close())

	s, err := NewServer(&cfg.PieceStorage)
	assert.NoError(t, err)
	port := "34897"
	startHTTPServer(ctx, t, port, s)

	wg := sync.WaitGroup{}
	requestAndCheck := func() {
		defer wg.Done()

		url := fmt.Sprintf("http://127.0.0.1:%s/piece/%s", port, pieceStr)
		req, err := http.NewRequest(http.MethodGet, url, nil)
		assert.NoError(t, err)
		resp, err := http.DefaultClient.Do(req)
		assert.NoError(t, err)
		defer resp.Body.Close() // nolint

		data, err := io.ReadAll(resp.Body)
		assert.NoError(t, err)
		assert.Equal(t, buf.Bytes(), data)
		fmt.Println("data length: ", len(data))
	}

	for i := 0; i < 10; i++ {
		wg.Add(3)
		go requestAndCheck()
		go requestAndCheck()
		go requestAndCheck()
	}
	wg.Wait()
}

func startHTTPServer(ctx context.Context, t *testing.T, port string, s *Server) {
	mux := mux.NewRouter()
	err := mux.HandleFunc("/piece/{cid}", s.RetrievalByPieceCID).GetError()
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
