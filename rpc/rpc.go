package rpc

import (
	"context"
	"encoding/hex"
	"fmt"
	"github.com/filecoin-project/go-jsonrpc"
	auth2 "github.com/filecoin-project/venus-auth/auth"
	"github.com/filecoin-project/venus-auth/cmd/jwtclient"
	"github.com/filecoin-project/venus-auth/core"
	"github.com/filecoin-project/venus-market/config"
	jwt3 "github.com/gbrlsnchs/jwt/v3"
	"github.com/gorilla/mux"
	logging "github.com/ipfs/go-log/v2"
	"github.com/multiformats/go-multiaddr"
	manet "github.com/multiformats/go-multiaddr/net"
	"golang.org/x/xerrors"
	"io/ioutil"
	"net/http"
	"path"
)

var log = logging.Logger("modules")

func ServeRPC(ctx context.Context, home config.IHome, cfg *config.API, api interface{}, shutdownCh <-chan struct{}, maxRequestSize int64, authUrl string) error {
	seckey, err := makeSecet(home, cfg)
	if err != nil {
		return err

	}
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

func makeSecet(cfg config.IHome, api *config.API) ([]byte, error) {
	var seckey []byte
	var token []byte
	var err error
	if len(api.Secret) == 0 {
		seckey, _, err = MakeToken()
		if err != nil {
			return nil, fmt.Errorf("make token failed:%s", err.Error())
		}
		api.Secret = hex.EncodeToString(seckey)
		err := config.SaveConfig(cfg)
		if err != nil {
			return nil, fmt.Errorf("save config failed:%s", err.Error())
		}
	} else {
		seckey, err = hex.DecodeString(api.Secret)
		if err != nil {
			return nil, xerrors.Errorf("unable to decode api security key")
		}
	}

	if token, err = jwt3.Sign(
		auth2.JWTPayload{
			Perm: core.PermAdmin,
			Name: "MarketLocalToken",
		}, jwt3.NewHS256(seckey)); err != nil {
		return nil, err
	}

	homePath, err := cfg.HomePath()
	if err != nil {
		return nil, xerrors.Errorf("unable to home path to save api/token")
	}
	_ = ioutil.WriteFile(path.Join(string(homePath), "api"), []byte(api.ListenAddress), 0644)
	_ = ioutil.WriteFile(path.Join(string(homePath), "token"), token, 0644)
	return seckey, nil
}
