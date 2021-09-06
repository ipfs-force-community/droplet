package rpc

import (
	"context"
	"fmt"
	"github.com/filecoin-project/go-jsonrpc"
	"github.com/filecoin-project/venus-auth/cmd/jwtclient"
	"github.com/filecoin-project/venus-market/config"
	"github.com/gorilla/mux"
	logging "github.com/ipfs/go-log/v2"
	"github.com/multiformats/go-multiaddr"
	manet "github.com/multiformats/go-multiaddr/net"
	"io/ioutil"
	"net/http"
	"path"
)

var log = logging.Logger("modules")

func ServeRPC(ctx context.Context, homeDir string, cfg *config.API, api interface{}, shutdownCh <-chan struct{}, maxRequestSize int64, authUrl string) error {
	seckey, token, err := MakeToken()
	if err != nil {
		return fmt.Errorf("make token failed:%s", err.Error())
	}

	_ = ioutil.WriteFile(path.Join(homeDir, "api"), []byte(cfg.ListenAddress), 0644)
	_ = ioutil.WriteFile(path.Join(homeDir, "token"), token, 0644)

	serverOptions := make([]jsonrpc.ServerOption, 0)
	if maxRequestSize != 0 { // config set
		serverOptions = append(serverOptions, jsonrpc.WithMaxRequestSize(maxRequestSize))
	}

	rpcServer := jsonrpc.NewServer(serverOptions...)
	rpcServer.Register("VENUS_MARKET", api)

	mux := mux.NewRouter()
	mux.Handle("/rpc/v0", rpcServer)
	mux.PathPrefix("/").Handler(http.DefaultServeMux)

	var handler http.Handler
	if len(authUrl) > 0 {
		cli := jwtclient.NewJWTClient(authUrl)
		handler = jwtclient.NewAuthMux(
			&localJwtClient{seckey: seckey}, jwtclient.WarpIJwtAuthClient(cli),
			mux, logging.Logger("auth"))
	} else {
		handler = jwtclient.NewAuthMux(
			&localJwtClient{seckey: seckey}, nil,
			mux, logging.Logger("auth"))
	}
	srv := &http.Server{Handler: handler}

	go func() {
		select {
		case <-shutdownCh:
		case <-ctx.Done():
		}
		log.Warn("Shutting down...")
		if err := srv.Shutdown(context.TODO()); err != nil {
			log.Errorf("shutting down RPC server failed: %s", err)
		}
		log.Warn("Graceful shutdown successful")
	}()

	addr, err := multiaddr.NewMultiaddr(cfg.ListenAddress)
	if err != nil {
		return err
	}

	nl, err := manet.Listen(addr)
	if err != nil {
		return err
	}
	log.Infof("start rpc listen %s", addr)
	return srv.Serve(manet.NetListener(nl))
}
