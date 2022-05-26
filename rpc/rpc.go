package rpc

import (
	"context"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"net/http"
	"path"

	"github.com/filecoin-project/go-jsonrpc"
	"github.com/multiformats/go-multiaddr"

	auth2 "github.com/filecoin-project/venus-auth/auth"
	"github.com/filecoin-project/venus-auth/cmd/jwtclient"
	"github.com/filecoin-project/venus-auth/core"
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

	secKey, err := makeSecret(cfg)
	if err != nil {
		return err
	}
	localJwtClient := NewJwtClient(secKey)
	token, err := localJwtClient.NewAuth(auth2.JWTPayload{
		Perm: core.PermAdmin,
		Name: "MarketLocalToken",
	})
	if err != nil {
		return err
	}
	if err = saveAPIInfo(home, cfg, secKey, token); err != nil {
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
		if err := srv.Shutdown(context.TODO()); err != nil && err != http.ErrServerClosed {
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

func makeSecret(apiCfg *config.API) ([]byte, error) {
	if len(apiCfg.Secret) != 0 {
		secret, err := hex.DecodeString(apiCfg.Secret)
		if err != nil {
			return nil, fmt.Errorf("unable to decode api security key")
		}
		return secret, nil
	}

	return RandSecret()
}

func saveAPIInfo(home config.IHome, apiCfg *config.API, secKey, token []byte) error {
	if len(apiCfg.Secret) == 0 {
		apiCfg.Secret = hex.EncodeToString(secKey)
		err := config.SaveConfig(home)
		if err != nil {
			return fmt.Errorf("save config failed:%s", err.Error())
		}
	}
	homePath, err := home.HomePath()
	if err != nil {
		return fmt.Errorf("unable to home path to save api/token")
	}
	_ = ioutil.WriteFile(path.Join(string(homePath), "api"), []byte(apiCfg.ListenAddress), 0644)
	_ = ioutil.WriteFile(path.Join(string(homePath), "token"), token, 0644)

	return nil
}
