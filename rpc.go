package main

import (
	"context"
	"fmt"
	"github.com/filecoin-project/go-jsonrpc"
	"github.com/filecoin-project/venus-auth/cmd/jwtclient"
	"github.com/filecoin-project/venus-market/api"
	"github.com/filecoin-project/venus-market/config"
	"github.com/gorilla/mux"
	logging "github.com/ipfs/go-log/v2"
	"github.com/multiformats/go-multiaddr"
	manet "github.com/multiformats/go-multiaddr/net"
	"net/http"
)

func serveRPC(ctx context.Context, cfg *config.API, a api.MarketNode, shutdownCh <-chan struct{}, maxRequestSize int64, authUrl string) error {
	seckey, err := MakeToken()
	if err != nil {
		return fmt.Errorf("make token failed:%s", err.Error())
	}

	serverOptions := make([]jsonrpc.ServerOption, 0)
	if maxRequestSize != 0 { // config set
		serverOptions = append(serverOptions, jsonrpc.WithMaxRequestSize(maxRequestSize))
	}

	rpcServer := jsonrpc.NewServer(serverOptions...)
	rpcServer.Register("Gateway", a)

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
	return srv.Serve(manet.NetListener(nl))
}
