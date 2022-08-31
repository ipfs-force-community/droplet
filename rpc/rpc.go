package rpc

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"path"

	"github.com/filecoin-project/go-jsonrpc"
	"github.com/multiformats/go-multiaddr"

	"github.com/filecoin-project/venus-auth/jwtclient"
	"github.com/filecoin-project/venus-market/v2/config"
	"github.com/gorilla/mux"
	logging "github.com/ipfs/go-log/v2"
	manet "github.com/multiformats/go-multiaddr/net"
)

var log = logging.Logger("modules")

func ServeRPC(ctx context.Context, home config.IHome, cfg *config.API, mux *mux.Router, maxRequestSize int64,
	namespace string, authClient *jwtclient.AuthClient, api interface{}, shutdownCh <-chan struct{}) error {
	serverOptions := make([]jsonrpc.ServerOption, 0)
	if maxRequestSize != 0 { // config set
		serverOptions = append(serverOptions, jsonrpc.WithMaxRequestSize(maxRequestSize))
	}

	rpcServer := jsonrpc.NewServer(serverOptions...)
	rpcServer.Register(namespace, api)
	mux.Handle("/rpc/v0", rpcServer)
	mux.PathPrefix("/").Handler(http.DefaultServeMux)

	localJwtClient, token, err := jwtclient.NewLocalAuthClient()
	if err != nil {
		return err
	}
	if err = saveAPIInfo(home, cfg, token); err != nil {
		return err
	}

	var handler http.Handler
	if authClient != nil {
		handler = jwtclient.NewAuthMux(localJwtClient, jwtclient.WarpIJwtAuthClient(authClient), mux)
	} else {
		handler = jwtclient.NewAuthMux(localJwtClient, nil, mux)
	}
	srv := &http.Server{Handler: handler}

	go func() {
		select {
		case <-shutdownCh:
		case <-ctx.Done():
		}
		log.Warn("RPC Shutting down...")
		if err := srv.Shutdown(context.Background()); err != nil && err != http.ErrServerClosed {
			log.Errorf("shutting down RPC server failed: %s", err)
		}
		log.Warn("RPC Graceful shutdown successful")
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

	if err := srv.Serve(manet.NetListener(nl)); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}

func saveAPIInfo(home config.IHome, apiCfg *config.API, token []byte) error {
	homePath, err := home.HomePath()
	if err != nil {
		return fmt.Errorf("unable to home path to save api/token")
	}
	_ = ioutil.WriteFile(path.Join(string(homePath), "api"), []byte(apiCfg.ListenAddress), 0644)
	_ = ioutil.WriteFile(path.Join(string(homePath), "token"), token, 0644)
	return nil
}
