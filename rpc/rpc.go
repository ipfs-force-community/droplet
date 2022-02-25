package rpc

import (
	"context"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"net/http"
	"path"

	"github.com/filecoin-project/go-jsonrpc"
	auth2 "github.com/filecoin-project/venus-auth/auth"
	"github.com/filecoin-project/venus-auth/cmd/jwtclient"
	"github.com/filecoin-project/venus-auth/core"
	"github.com/filecoin-project/venus-market/config"
	"github.com/gorilla/mux"
	logging "github.com/ipfs/go-log/v2"
	"github.com/multiformats/go-multiaddr"
	manet "github.com/multiformats/go-multiaddr/net"
	"golang.org/x/xerrors"
)

var log = logging.Logger("modules")

func ServeRPC(ctx context.Context, home config.IHome, cfg *config.API, mux *mux.Router, maxRequestSize int64, namespace string, authUrl string, api interface{}, shutdownCh <-chan struct{}) error {
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
	if len(authUrl) > 0 {
		cli := jwtclient.NewJWTClient(authUrl)
		handler = jwtclient.NewAuthMux(localJwtClient, jwtclient.WarpIJwtAuthClient(cli), mux, logging.Logger("auth"))
	} else {
		handler = jwtclient.NewAuthMux(localJwtClient, nil, mux, logging.Logger("auth"))
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

func makeSecret(apiCfg *config.API) ([]byte, error) {
	if len(apiCfg.Secret) != 0 {
		secret, err := hex.DecodeString(apiCfg.Secret)
		if err != nil {
			return nil, xerrors.Errorf("unable to decode api security key")
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
		return xerrors.Errorf("unable to home path to save api/token")
	}
	_ = ioutil.WriteFile(path.Join(string(homePath), "api"), []byte(apiCfg.ListenAddress), 0644)
	_ = ioutil.WriteFile(path.Join(string(homePath), "token"), token, 0644)

	return nil
}
