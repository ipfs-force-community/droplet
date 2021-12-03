package rpc

import (
	"fmt"
	"github.com/filecoin-project/venus-market/piecestorage"
	"io"
	"net/http"
	"strconv"
)

var _ http.Handler = (*PieceStorageServer)(nil)

type PieceStorageServer struct {
	pieceStorage piecestorage.IPieceStorage
}

func NewPieceStorageServer(pieceStorage piecestorage.IPieceStorage) *PieceStorageServer {
	return &PieceStorageServer{pieceStorage: pieceStorage}
}

func (p *PieceStorageServer) ServeHTTP(res http.ResponseWriter, req *http.Request) {
	resourceId := req.URL.Query().Get("resource-id")
	if len(resourceId) > 0 {
		res.Write([]byte("resource is empty"))
		res.WriteHeader(http.StatusBadRequest)
		return
	}

	has, err := p.pieceStorage.Has(req.Context(), resourceId)
	if err != nil {
		res.Write([]byte(fmt.Sprintf("error to read resource %s %s", resourceId, err)))
		res.WriteHeader(http.StatusInternalServerError)
		return
	}

	if !has {
		res.Write([]byte(fmt.Sprintf("resource %s not found ", resourceId)))
		res.WriteHeader(http.StatusInternalServerError)
		return
	}

	flen, err := p.pieceStorage.Len(req.Context(), resourceId)
	if err != nil {
		res.Write([]byte(fmt.Sprintf("error to read resource %s %s", resourceId, err)))
		res.WriteHeader(http.StatusInternalServerError)
		return
	}

	r, err := p.pieceStorage.Read(req.Context(), resourceId)
	if err != nil {
		res.Write([]byte(fmt.Sprintf("error to read resource %s %s", resourceId, err)))
		res.WriteHeader(http.StatusInternalServerError)
		return
	}

	res.Header().Set("Content-Length", strconv.FormatInt(flen, 10))
	_, err = io.Copy(res, r)
	if err != nil {
		res.Write([]byte(fmt.Sprintf("error to read resource %s %s", resourceId, err)))
		res.WriteHeader(http.StatusInternalServerError)
		return
	}
}
