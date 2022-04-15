package rpc

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"path"
	"strings"
	"sync"
	"testing"

	"github.com/gorilla/mux"

	"github.com/stretchr/testify/require"
)

const httpPort = 8081

var dummyDealsBase = fmt.Sprintf("http://localhost:%d/"+DealsPrefix, httpPort)

func TestDummyServer(t *testing.T) {
	rq := require.New(t)
	ctx := context.Background()

	mux := mux.NewRouter()
	listenAddr := fmt.Sprintf(":%d", httpPort)
	t.Logf("server listening on %s\n", listenAddr)
	err := DealServer(mux, DefDealsDir)
	rq.NoError(err)

	srv := &http.Server{Addr: listenAddr, Handler: mux}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()

		if err := srv.ListenAndServe(); err != http.ErrServerClosed {
			rq.Fail(err.Error())
		}
	}()

	var sourceBuff bytes.Buffer
	sourceBuff.WriteString(strings.Repeat("test\n", 1024))
	randSource := &sourceBuff
	var randBytes bytes.Buffer
	_, err = randBytes.ReadFrom(randSource)
	rq.NoError(err)

	fileName := "test.car"
	err = ioutil.WriteFile(path.Join(DefDealsDir, fileName), randBytes.Bytes(), 0644)
	rq.NoError(err)

	reqUrl := dummyDealsBase + "/" + fileName
	req, err := http.NewRequest("GET", reqUrl, nil)
	rq.NoError(err)

	req = req.WithContext(ctx)
	resp, err := http.DefaultClient.Do(req)
	rq.NoError(err)

	defer resp.Body.Close() // nolint
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		rq.Fail(fmt.Errorf("http req failed: code:%d, status:%s", resp.StatusCode, resp.Status).Error())
	}

	var buf bytes.Buffer
	_, err = buf.ReadFrom(resp.Body)
	rq.NoError(err)

	rq.Equal(randBytes.String(), buf.String())

	t.Logf("Read %d bytes", len(buf.Bytes()))

	if err := srv.Shutdown(ctx); err != nil {
		rq.Fail(err.Error())
	}

	wg.Wait()
}
