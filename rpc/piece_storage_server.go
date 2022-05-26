package rpc

import (
	"fmt"
	"io"

	"github.com/filecoin-project/venus-market/v2/piecestorage"
	logging "github.com/ipfs/go-log/v2"

	"net/http"
	"strconv"
)

var resourceLog = logging.Logger("resource")

var _ http.Handler = (*PieceStorageServer)(nil)

type PieceStorageServer struct {
	pieceStorageMgr *piecestorage.PieceStorageManager
}

func NewPieceStorageServer(pieceStorageMgr *piecestorage.PieceStorageManager) *PieceStorageServer {
	return &PieceStorageServer{pieceStorageMgr: pieceStorageMgr}
}

func (p *PieceStorageServer) ServeHTTP(res http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		logErrorAndResonse(res, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}

	resourceID := req.URL.Query().Get("resource-id")
	if len(resourceID) == 0 {
		logErrorAndResonse(res, "resource is empty", http.StatusBadRequest)
		return
	}
	ctx := req.Context()

	//todo consider priority strategy, priority oss, priority market transfer directly
	pieceStorage, err := p.pieceStorageMgr.FindStorageForRead(ctx, resourceID)
	if err != nil {
		logErrorAndResonse(res, fmt.Sprintf("resource %s not found", resourceID), http.StatusNotFound)
		return
	}

	redirectUrl, err := pieceStorage.GetRedirectUrl(ctx, resourceID)
	if err != nil && err != piecestorage.ErrUnsupportRedirect {
		logErrorAndResonse(res, fmt.Sprintf("fail to get redirect url of piece  %s: %s", resourceID, err), http.StatusInternalServerError)
		return
	}

	if err == nil {
		res.Header().Set("Location", redirectUrl)
		res.WriteHeader(http.StatusFound)
		return
	}

	flen, err := pieceStorage.Len(req.Context(), resourceID)
	if err != nil {
		logErrorAndResonse(res, fmt.Sprintf("call piecestore.Len for %s: %s", resourceID, err), http.StatusInternalServerError)
		return
	}
	res.Header().Set("Content-Length", strconv.FormatInt(flen, 10))

	r, err := pieceStorage.GetReaderCloser(req.Context(), resourceID)
	if err != nil {
		logErrorAndResonse(res, fmt.Sprintf("failed to open reader for %s: %s", resourceID, err), http.StatusInternalServerError)
		return
	}

	defer func() {
		if err = r.Close(); err != nil {
			log.Errorf("unable to close http %v", err)
		}
	}()

	// TODO:
	// as we can not override http response headers after body transfer has began
	// we can only log the error info here
	_, _ = io.Copy(res, r)
}

func logErrorAndResonse(res http.ResponseWriter, err string, code int) {
	resourceLog.Errorf("resource request fail Code: %d, Message: %s", code, err)
	http.Error(res, err, code)
}
