package rpc

import (
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/filecoin-project/venus-market/piecestorage"
)

var _ http.Handler = (*PieceStorageServer)(nil)

type PieceStorageServer struct {
	pieceStorageMgr *piecestorage.PieceStorageManager
}

func NewPieceStorageServer(pieceStorageMgr *piecestorage.PieceStorageManager) *PieceStorageServer {
	return &PieceStorageServer{pieceStorageMgr: pieceStorageMgr}
}

func (p *PieceStorageServer) ServeHTTP(res http.ResponseWriter, req *http.Request) {
	resourceID := req.URL.Query().Get("resource-id")
	if len(resourceID) == 0 {
		http.Error(res, "resource is empty", http.StatusBadRequest)
		return
	}

	pieceStorage, err := p.pieceStorageMgr.FindStorageForRead(req.Context(), resourceID)
	if err != nil {
		http.Error(res, fmt.Sprintf("resource %s not found", resourceID), http.StatusNotFound)
		return
	}

	flen, err := pieceStorage.Len(req.Context(), resourceID)
	if err != nil {
		http.Error(res, fmt.Sprintf("call piecestore.Len for %s: %s", resourceID, err), http.StatusInternalServerError)
		return
	}

	r, err := pieceStorage.GetReaderCloser(req.Context(), resourceID)
	if err != nil {
		http.Error(res, fmt.Sprintf("failed to open reader for %s: %s", resourceID, err), http.StatusInternalServerError)
		return
	}

	defer func() {
		if err = r.Close(); err != nil {
			log.Errorf("unable to close http %v", err)
		}
	}()

	res.Header().Set("Content-Length", strconv.FormatInt(flen, 10))
	// TODO:
	// as we can not override http response headers after body transfer has began
	// we can only log the error info here
	_, _ = io.Copy(res, r)
}
