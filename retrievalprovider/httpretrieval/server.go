package httpretrieval

import (
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/textproto"
	"strconv"
	"strings"
	"time"

	"github.com/NYTimes/gziphandler"
	"github.com/filecoin-project/go-fil-markets/storagemarket"
	"github.com/filecoin-project/go-fil-markets/stores"
	"github.com/filecoin-project/go-padreader"
	"github.com/filecoin-project/go-state-types/abi"
	marketAPI "github.com/filecoin-project/venus/venus-shared/api/market/v1"
	"github.com/filecoin-project/venus/venus-shared/types"
	marketTypes "github.com/filecoin-project/venus/venus-shared/types/market"
	"github.com/ipfs-force-community/droplet/v2/piecestorage"
	"github.com/ipfs/go-cid"
	logging "github.com/ipfs/go-log/v2"
	"go.uber.org/zap"
)

const (
	pieceBasePath = "/piece/"
	ipfsBasePath  = "/ipfs/"
)

// errNoOverlap is returned by serveContent's parseRange if first-byte-pos of
// all of the byte-range-spec values is greater than the content size.
var errNoOverlap = errors.New("invalid range: failed to overlap")

var log = logging.Logger("httpserver")

type Server struct {
	pieceMgr         *piecestorage.PieceStorageManager
	api              marketAPI.IMarket
	trustlessHandler *trustlessHandler
	compressionLevel int
}

func NewServer(ctx context.Context,
	pieceMgr *piecestorage.PieceStorageManager,
	api marketAPI.IMarket,
	dagStoreWrapper stores.DAGStoreWrapper,
	compressionLevel int,
) (*Server, error) {
	tlHandler := newTrustlessHandler(ctx, newBSWrap(ctx, dagStoreWrapper), gzip.BestSpeed)
	return &Server{pieceMgr: pieceMgr, api: api, trustlessHandler: tlHandler, compressionLevel: compressionLevel}, nil
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, ipfsBasePath) {
		s.retrievalByIPFS(w, r)
		return
	}

	s.pieceHandler()(w, r)
}

func (s *Server) pieceHandler() http.HandlerFunc {
	var pieceHandler http.Handler = http.HandlerFunc(s.retrievalByPieceCID)
	if s.compressionLevel != gzip.NoCompression {
		gzipWrapper := gziphandler.MustNewGzipLevelHandler(s.compressionLevel)
		pieceHandler = gzipWrapper(pieceHandler)
		log.Debugf("enabling compression with a level of %d", s.compressionLevel)
	}
	return pieceHandler.ServeHTTP
}

func (s *Server) retrievalByIPFS(w http.ResponseWriter, r *http.Request) {
	s.trustlessHandler.ServeHTTP(w, r)
}

func (s *Server) retrievalByPieceCID(w http.ResponseWriter, r *http.Request) {
	pieceCID, err := convertPieceCID(r.URL.Path)
	if err != nil {
		log.Warn(err)
		badResponse(w, http.StatusBadRequest, err)
		return
	}

	ctx := r.Context()
	pieceCIDStr := pieceCID.String()
	log := log.With("piece cid", pieceCIDStr)
	log.Infof("start retrieval deal, Range: %s", r.Header.Get("Range"))

	_, err = s.listDealsByPiece(ctx, pieceCIDStr)
	if err != nil {
		log.Warn(err)
		// todo: reject request?
		// badResponse(w, http.StatusNotFound, err)
		// return
	}

	store, err := s.pieceMgr.FindStorageForRead(ctx, pieceCIDStr)
	if err != nil {
		log.Warn(err)
		// if errors.Is(err, piecestorage.ErrorNotFoundForRead) {
		// todo: unseal data
		// }
		badResponse(w, http.StatusNotFound, err)
		return
	}
	len, err := store.Len(ctx, pieceCIDStr)
	if err != nil {
		log.Warn(err)
		badResponse(w, http.StatusInternalServerError, err)
		return
	}
	log.Infof("piece size: %v", len)

	mountReader, err := store.GetMountReader(ctx, pieceCIDStr)
	if err != nil {
		log.Warn(err)
		badResponse(w, http.StatusInternalServerError, err)
		return
	}
	defer mountReader.Close() // nolint

	contentReader, err := handleRangeHeader(r.Header.Get("Range"), mountReader, len)
	if err != nil {
		log.Warnf("handleRangeHeader failed, Range: %s, error: %v", r.Header.Get("Range"), err)
		badResponse(w, http.StatusInternalServerError, err)
		return
	}
	setHeaders(w, pieceCID)
	serveContent(w, r, contentReader, log)
	log.Info("end retrieval deal")
}

func (s *Server) listDealsByPiece(ctx context.Context, piece string) ([]marketTypes.MinerDeal, error) {
	activeState := storagemarket.StorageDealActive
	p := &marketTypes.StorageDealQueryParams{
		PieceCID: piece,
		Page:     marketTypes.Page{Limit: 100},
		State:    &activeState,
	}
	deals, err := s.api.MarketListIncompleteDeals(ctx, p)
	if err != nil {
		return nil, err
	}
	if len(deals) == 0 {
		return nil, fmt.Errorf("not found deal")
	}

	return deals, nil
}

func isGzipped(res http.ResponseWriter) bool {
	switch res.(type) {
	case *gziphandler.GzipResponseWriter, gziphandler.GzipResponseWriterWithCloseNotify:
		// there are conditions where we may have a GzipResponseWriter but the
		// response will not be compressed, but they are related to very small
		// response sizes so this shouldn't matter (much)
		return true
	}
	return false
}

func setHeaders(w http.ResponseWriter, pieceCid cid.Cid) {
	w.Header().Set("Vary", "Accept-Encoding")
	etag := `"` + pieceCid.String() + `"` // must be quoted
	if isGzipped(w) {
		etag = etag[:len(etag)-1] + ".gz\""
	}
	w.Header().Set("Etag", etag)
	w.Header().Set("Content-Type", "application/piece")
	w.Header().Set("Cache-Control", "public, max-age=29030400, immutable")
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
	total, count := writeErrWatcher.total, writeErrWatcher.count
	var avg uint64
	if count != 0 {
		avg = total / count
	}

	completeMsg := fmt.Sprintf("GET %s\t%s - %s: %s / %s transferred",
		r.URL, end.Format(time.RFC3339), start.Format(time.RFC3339), time.Since(start),
		fmt.Sprintf("total %s (%d B), average write %s ", types.SizeStr(types.NewInt(total)), total, types.SizeStr(types.NewInt(avg))))
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
	l := len(pieceBasePath)
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

func handleRangeHeader(r string, mountReader io.ReadSeeker, carSize int64) (io.ReadSeeker, error) {
	paddedSize := padreader.PaddedSize(uint64(carSize))
	if paddedSize == abi.UnpaddedPieceSize(carSize) {
		return mountReader, nil
	}

	ranges, err := parseRange(r, int64(paddedSize))
	if err != nil {
		return nil, err
	}

	for _, r := range ranges {
		if r[0]+r[1] >= carSize {
			return newMultiReader(mountReader, uint64(carSize)), nil
		}
	}

	return mountReader, nil
}

// parseRange parses a Range header string as per RFC 7233.
// errNoOverlap is returned if none of the ranges overlap.
func parseRange(s string, size int64) ([][2]int64, error) {
	if s == "" {
		return nil, nil // header not present
	}
	const b = "bytes="
	if !strings.HasPrefix(s, b) {
		return nil, errors.New("invalid range")
	}
	var ranges [][2]int64
	noOverlap := false
	for _, ra := range strings.Split(s[len(b):], ",") {
		ra = textproto.TrimString(ra)
		if ra == "" {
			continue
		}
		start, end, ok := strings.Cut(ra, "-")
		if !ok {
			return nil, errors.New("invalid range")
		}
		start, end = textproto.TrimString(start), textproto.TrimString(end)
		r := [2]int64{}
		if start == "" {
			// If no start is specified, end specifies the
			// range start relative to the end of the file,
			// and we are dealing with <suffix-length>
			// which has to be a non-negative integer as per
			// RFC 7233 Section 2.1 "Byte-Ranges".
			if end == "" || end[0] == '-' {
				return nil, errors.New("invalid range")
			}
			i, err := strconv.ParseInt(end, 10, 64)
			if i < 0 || err != nil {
				return nil, errors.New("invalid range")
			}
			if i > size {
				i = size
			}
			r[0] = size - i
			r[1] = size - r[0]
		} else {
			i, err := strconv.ParseInt(start, 10, 64)
			if err != nil || i < 0 {
				return nil, errors.New("invalid range")
			}
			if i >= size {
				// If the range begins after the size of the content,
				// then it does not overlap.
				noOverlap = true
				continue
			}
			r[0] = i
			if end == "" {
				// If no end is specified, range extends to end of the file.
				r[1] = size - r[0]
			} else {
				i, err := strconv.ParseInt(end, 10, 64)
				if err != nil || r[0] > i {
					return nil, errors.New("invalid range")
				}
				if i >= size {
					i = size - 1
				}
				r[1] = i - r[0] + 1
			}
		}
		ranges = append(ranges, r)
	}
	if noOverlap && len(ranges) == 0 {
		// The specified ranges did not overlap with the content.
		return nil, errNoOverlap
	}
	return ranges, nil
}

// writeErrorWatcher calls onError if there is an error writing to the writer
type writeErrorWatcher struct {
	http.ResponseWriter
	total, count uint64
	onError      func(err error)
}

func (w *writeErrorWatcher) Write(bz []byte) (int, error) {
	n, err := w.ResponseWriter.Write(bz)
	if err != nil {
		w.onError(err)
	}
	w.total += uint64(n)
	w.count++
	return n, err
}
