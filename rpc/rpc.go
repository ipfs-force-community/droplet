package rpc

import (
	"context"
	"encoding/hex"
	"fmt"
	"net/http"
	_ "net/http/pprof"
	"os"
	"path"
	"regexp"

	"github.com/etherlabsio/healthcheck/v2"
	"github.com/filecoin-project/go-jsonrpc"
	"github.com/gorilla/mux"
	logging "github.com/ipfs/go-log/v2"
	"github.com/multiformats/go-multiaddr"
	manet "github.com/multiformats/go-multiaddr/net"

	authconfig "github.com/ipfs-force-community/sophon-auth/config"
	"github.com/ipfs-force-community/sophon-auth/jwtclient"

	"github.com/ipfs-force-community/droplet/v2/config"
	"github.com/ipfs-force-community/droplet/v2/retrievalprovider/httpretrieval"
)

var log = logging.Logger("modules")

type APIHandle struct {
	Path string
	API  interface{}
}

func ServeRPC(
	ctx context.Context,
	home config.IHome,
	apiCfg *config.API,
	mux *mux.Router,
	maxRequestSize int64,
	namespace string,
	authClient *jwtclient.AuthClient,
	apiHandles []APIHandle,
	shutdownCh <-chan struct{},
	httpRetrievalServer *httpretrieval.Server,
) error {
	serverOptions := make([]jsonrpc.ServerOption, 0)
	if maxRequestSize != 0 { // config set
		serverOptions = append(serverOptions, jsonrpc.WithMaxRequestSize(maxRequestSize))
	}

	serveRpc := func(path string, hnd interface{}) {
		rpcServer := jsonrpc.NewServer(serverOptions...)
		rpcServer.Register(namespace, hnd)
		mux.Handle(path, rpcServer)
	}

	for _, apiHnd := range apiHandles {
		serveRpc(apiHnd.Path, apiHnd.API)
	}

	mux.PathPrefix("/").Handler(http.DefaultServeMux)

	localJwtClient, err := getLocalJwtClient(home, apiCfg)
	if err != nil {
		return err
	}
	var authMux *jwtclient.AuthMux
	if authClient != nil {
		authMux = jwtclient.NewAuthMux(localJwtClient, jwtclient.WarpIJwtAuthClient(authClient), mux)
	} else {
		authMux = jwtclient.NewAuthMux(localJwtClient, nil, mux)
	}
	authMux.TrustHandle("/healthcheck", healthcheck.Handler())
	authMux.TrustHandle("/debug/pprof/", http.DefaultServeMux)
	if httpRetrievalServer != nil {
		authMux.TrustHandle("/piece/", httpRetrievalServer, jwtclient.RegexpOption(regexp.MustCompile(`/piece/[a-z0-9]+`)))
		authMux.TrustHandle("/ipfs/", httpRetrievalServer, jwtclient.RegexpOption(regexp.MustCompile(`/ipfs/[a-z0-9]+`)))
	}

	srv := &http.Server{Handler: authMux}

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

	addr, err := multiaddr.NewMultiaddr(apiCfg.ListenAddress)
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

func getLocalJwtClient(home config.IHome, apiCfg *config.API) (jwtclient.IJwtAuthClient, error) {
	if len(apiCfg.PrivateKey) == 0 {
		secret, err := authconfig.RandSecret()
		if err != nil {
			return nil, err
		}
		apiCfg.PrivateKey = hex.EncodeToString(secret)
		err = config.SaveConfig(home)
		if err != nil {
			return nil, err
		}
	}

	secret, err := hex.DecodeString(apiCfg.PrivateKey)
	if err != nil {
		return nil, err
	}

	localJwtClient, token, err := jwtclient.NewLocalAuthClientWithSecret(secret)
	if err != nil {
		return nil, err
	}

	err = saveAPIInfo(home, apiCfg, token)
	if err != nil {
		return nil, err
	}
	return localJwtClient, nil
}

func saveAPIInfo(home config.IHome, apiCfg *config.API, token []byte) error {
	homePath, err := home.HomePath()
	if err != nil {
		return fmt.Errorf("unable to home path to save api/token")
	}
	_ = os.WriteFile(path.Join(string(homePath), "api"), []byte(apiCfg.ListenAddress), 0o644)
	_ = os.WriteFile(path.Join(string(homePath), "token"), token, 0o644)
	return nil
}
