package rpc

import (
	"bytes"
	"context"
	"fmt"

	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/filecoin-project/venus-market/v2/config"
	"github.com/filecoin-project/venus-market/v2/piecestorage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestServer(t *testing.T) (*piecestorage.MemPieceStore, http.Handler) {
	pm, err := piecestorage.NewPieceStorageManager(&config.PieceStorage{})
	require.NoError(t, err)
	ps := piecestorage.NewMemPieceStore("memtest", nil)
	pm.AddMemPieceStorage(ps)
	pss := NewPieceStorageServer(pm)
	return ps, pss
}

func TestResouceDownload(t *testing.T) {
	ctx := context.Background()
	ps, psm := setupTestServer(t)

	resourceId := "s1"
	_, err := ps.SaveTo(ctx, resourceId, bytes.NewBufferString("mock resource1 content"))
	assert.Nil(t, err)
	ps.RedirectResources[resourceId] = true

	resourceId2 := "s2"
	_, err = ps.SaveTo(ctx, resourceId2, bytes.NewBufferString("mock resource2 content"))
	assert.Nil(t, err)
	ps.RedirectResources[resourceId2] = false

	t.Run("redirect", func(t *testing.T) {
		path := fmt.Sprintf("http://127.0.0.1:3030?resource-id=%s", resourceId)
		req := httptest.NewRequest(http.MethodGet, path, nil)
		w := httptest.NewRecorder()
		psm.ServeHTTP(w, req)

		assert.Equal(t, http.StatusFound, w.Code)
		if val, ok := w.Header()["Location"]; ok {
			assert.Equal(t, fmt.Sprintf("mock redirect resourceId %s", resourceId), val[0])
		} else {
			assert.FailNow(t, "expect redirect header but not found")
		}
	})

	t.Run("download directly", func(t *testing.T) {
		path := fmt.Sprintf("http://127.0.0.1:3030?resource-id=%s", resourceId2)
		req := httptest.NewRequest(http.MethodGet, path, nil)
		w := httptest.NewRecorder()
		psm.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		result, err := ioutil.ReadAll(w.Body)
		assert.Nil(t, err)
		assert.Equal(t, "mock resource2 content", string(result))
	})

}
