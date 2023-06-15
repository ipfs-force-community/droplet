package httpretrieval

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/NYTimes/gziphandler"
	"github.com/filecoin-project/venus-market/v2/config"
	"github.com/filecoin-project/venus-market/v2/piecestorage"
	"github.com/filecoin-project/venus/venus-shared/types"
	"github.com/ipfs/go-cid"
	logging "github.com/ipfs/go-log/v2"
	"go.uber.org/zap"
)

var log = logging.Logger("httpserver")

type Server struct {
	// path     string
	pieceMgr *piecestorage.PieceStorageManager
}

func NewServer(cfg *config.PieceStorage) (*Server, error) {
	pieceMgr, err := piecestorage.NewPieceStorageManager(cfg)
	if err != nil {
		return nil, err
	}

	return &Server{pieceMgr: pieceMgr}, nil
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.RetrievalByPieceCID(w, r)
}

func (s *Server) RetrievalByPieceCID(w http.ResponseWriter, r *http.Request) {
	pieceCID, err := convertPieceCID(r.URL.Path)
	if err != nil {
		log.Warn(err)
		badResponse(w, http.StatusBadRequest, err)
		return
	}

	ctx := r.Context()
	pieceCIDStr := pieceCID.String()
	log := log.With("piece cid", pieceCIDStr)
	log.Info("start retrieval deal")
	store, err := s.pieceMgr.FindStorageForRead(ctx, pieceCIDStr)
	if err != nil {
		log.Warn(err)
		badResponse(w, http.StatusNotFound, err)
		return
	}
	mountReader, err := store.GetMountReader(ctx, pieceCIDStr)
	if err != nil {
		log.Warn(err)
		badResponse(w, http.StatusNotFound, err)
		return
	}

	serveContent(w, r, mountReader, log)
	log.Info("end retrieval deal")
}

func serveContent(w http.ResponseWriter, r *http.Request, content io.ReadSeeker, log *zap.SugaredLogger) {
	// Set the Content-Type header explicitly so that http.ServeContent doesn't
	// try to do it implicitly
	w.Header().Set("Content-Type", "application/piece")

	var writer http.ResponseWriter

	// http.ServeContent ignores errors when writing to the stream, so we
	// replace the writer with a class that watches for errors
	var err error
	writeErrWatcher := &writeErrorWatcher{ResponseWriter: w, onError: func(e error) {
		err = e
	}}

	writer = writeErrWatcher //Need writeErrWatcher to be of type writeErrorWatcher for addCommas()

	// Note that the last modified time is a constant value because the data
	// in a piece identified by a cid will never change.
	start := time.Now()
	log.Infof("start %s\t %d\tGET %s", start, http.StatusOK, r.URL)
	isGzipped := strings.Contains(r.Header.Get("Accept-Encoding"), "gzip")
	if isGzipped {
		// If Accept-Encoding header contains gzip then send a gzipped response
		gzwriter := gziphandler.GzipResponseWriter{
			ResponseWriter: writeErrWatcher,
		}
		// Close the writer to flush buffer
		defer gzwriter.Close() // nolint
		writer = &gzwriter
	}

	if r.Method == "HEAD" {
		// For an HTTP HEAD request ServeContent doesn't send any data (just headers)
		http.ServeContent(writer, r, "", time.Time{}, content)
		log.Infof("%d\tHEAD %s", http.StatusOK, r.URL)
		return
	}

	// Send the content
	http.ServeContent(writer, r, "", time.Unix(1, 0), content)

	// Write a line to the log
	end := time.Now()
	completeMsg := fmt.Sprintf("GET %s\t%s - %s: %s / %s transferred",
		r.URL, end.Format(time.RFC3339), start.Format(time.RFC3339), time.Since(start),
		fmt.Sprintf("%s (%d B)", types.SizeStr(types.NewInt(writeErrWatcher.count)), writeErrWatcher.count))
	if isGzipped {
		completeMsg += " (gzipped)"
	}
	if err == nil {
		log.Infof("%s %s", completeMsg, "Done")
	} else {
		log.Warnf("%s %s\n%s", completeMsg, "FAIL", err)
	}
}

func convertPieceCID(path string) (cid.Cid, error) {
	l := len("/piece/")
	if len(path) <= l {
		return cid.Undef, fmt.Errorf("path %s too short", path)
	}

	cidStr := path[l:]
	c, err := cid.Parse(cidStr)
	if err != nil {
		return cid.Undef, fmt.Errorf("parse piece cid failed: %s, %v", cidStr, err)
	}

	return c, nil
}

func badResponse(w http.ResponseWriter, code int, err error) {
	w.WriteHeader(code)
	w.Write([]byte("Error: " + err.Error())) // nolint
}

// writeErrorWatcher calls onError if there is an error writing to the writer
type writeErrorWatcher struct {
	http.ResponseWriter
	count   uint64
	onError func(err error)
}

func (w *writeErrorWatcher) Write(bz []byte) (int, error) {
	count, err := w.ResponseWriter.Write(bz)
	if err != nil {
		w.onError(err)
	}
	w.count += uint64(count)
	return count, err
}
