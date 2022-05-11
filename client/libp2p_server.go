package client

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strconv"

	"github.com/google/uuid"
	"github.com/ipfs-force-community/venus-common-utils/metrics"
	logging "github.com/ipfs/go-log/v2"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	gostream "github.com/libp2p/go-libp2p-gostream"
	"golang.org/x/xerrors"

	"github.com/filecoin-project/venus-market/v2/imports"
	"github.com/filecoin-project/venus-market/v2/types"
)

var libp2pLog = logging.Logger("libp2p-server")

type Libp2pServer struct {
	ctx             context.Context
	cancel          context.CancelFunc
	h               host.Host
	listener        net.Listener
	server          *http.Server
	authTokenDB     *AuthTokenDB
	clientImportMgr *imports.Manager
}

func NewLibp2pServer(ctx metrics.MetricsCtx, h host.Host, authTokenDB *AuthTokenDB, clientImportMgr ClientImportMgr) (*Libp2pServer, error) {
	s := &Libp2pServer{
		h:               h,
		authTokenDB:     authTokenDB,
		clientImportMgr: clientImportMgr,
	}
	s.ctx, s.cancel = context.WithCancel(ctx)

	// Listen on HTTP over libp2p
	listener, err := gostream.Listen(h, types.DataTransferProtocol)
	if err != nil {
		return nil, xerrors.Errorf("starting gostream listener: %w", err)
	}
	s.listener = listener

	return s, nil
}

func (s *Libp2pServer) Start() {
	handler := http.NewServeMux()
	handler.HandleFunc("/", s.Handler)
	s.server = &http.Server{
		Handler: handler,
		BaseContext: func(listener net.Listener) context.Context {
			return s.ctx
		},
	}
	go s.server.Serve(s.listener) //nolint:errcheck
}

func (s *Libp2pServer) Stop() {
	s.cancel()
}

func (s *Libp2pServer) Handler(w http.ResponseWriter, r *http.Request) {
	_, authVal, herr := s.checkAuth(r)
	if herr != nil {
		libp2pLog.Infow("data transfer request failed", "code", herr.code, "err", herr.error, "peer", r.RemoteAddr)
		w.WriteHeader(herr.code)
		return
	}

	// Get the peer ID from the RemoteAddr
	pid, err := peer.Decode(r.RemoteAddr)
	if err != nil {
		libp2pLog.Infow("data transfer request failed: parsing remote address as peer ID",
			"remote-addr", r.RemoteAddr, "err", err)
		http.Error(w, "Failed to parse remote address '"+r.RemoteAddr+"' as peer ID", http.StatusBadRequest)
		return
	}

	// Protect the libp2p connection for the lifetime of the transfer
	tag := uuid.New().String()
	s.h.ConnManager().Protect(pid, tag)
	defer s.h.ConnManager().Unprotect(pid, tag)

	carPath, err := s.clientImportMgr.CARPathFor(context.Background(), authVal.PayloadCid)
	if err != nil {
		errMsg := fmt.Sprintf("Failed to found car path %v error %v", authVal.PayloadCid, err)
		libp2pLog.Error(errMsg)
		http.Error(w, errMsg, http.StatusInternalServerError)
		return
	}
	fileInfo, err := os.Stat(carPath)
	if err != nil {
		errMsg := fmt.Sprintf("Failed to stat car file %s error %v", carPath, err)
		libp2pLog.Error(errMsg)
		http.Error(w, errMsg, http.StatusInternalServerError)
	}
	if fileInfo.Size() != int64(authVal.Size) {
		errMsg := fmt.Sprintf("File size not match, file name %s excepted %d acutal %d", carPath, authVal.Size, fileInfo.Size())
		libp2pLog.Error(errMsg)
		http.Error(w, errMsg, http.StatusInternalServerError)
	}

	f, err := os.Open(carPath)
	if err != nil {
		errMsg := fmt.Sprintf("Failed to open car file %s error %v", carPath, err)
		libp2pLog.Error(errMsg)
		http.Error(w, errMsg, http.StatusInternalServerError)
		return
	}
	defer f.Close() // nolint:errcheck

	w.Header().Set("Content-Length", strconv.FormatInt(int64(authVal.Size), 10))

	_, err = io.Copy(w, bufio.NewReader(f))
	if err != nil {
		libp2pLog.Errorf("call io.Copy failed %v %v", authVal.ProposalCid, err)
	}
}

type httpError struct {
	error
	code int
}

func (s *Libp2pServer) checkAuth(r *http.Request) (string, *types.AuthValue, *httpError) {
	ctx := r.Context()

	// Get auth token from Authorization header
	_, authToken, ok := r.BasicAuth()
	if !ok {
		return "", nil, &httpError{
			error: xerrors.New("rejected request with no Authorization header"),
			code:  http.StatusUnauthorized,
		}
	}

	// Get auth value from auth datastore
	val, err := s.authTokenDB.Get(ctx, authToken)
	if xerrors.Is(err, types.ErrTokenNotFound) {
		return "", nil, &httpError{
			error: xerrors.New("rejected unrecognized auth token"),
			code:  http.StatusUnauthorized,
		}
	} else if err != nil {
		return "", nil, &httpError{
			error: fmt.Errorf("getting key from datastore: %w", err),
			code:  http.StatusInternalServerError,
		}
	}

	return authToken, val, nil
}
