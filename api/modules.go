package api

import (
	"context"
	logging "github.com/ipfs/go-log/v2"
	"go.uber.org/fx"
	"net"
	"net/http"
)

var log = logging.Logger("net")

func RunAPI(lc fx.Lifecycle, lst net.Listener) error {
	apiserv := &http.Server{
		//Handler: handler,
	}

	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			go func() {
				log.Info("Start rpcserver ", lst.Addr())
				if err := apiserv.Serve(lst); err != nil {
					log.Errorf("Start rpcserver failed: %v", err)
				}
			}()
			return nil
		},
		OnStop: func(ctx context.Context) error {
			return lst.Close()
		},
	})
	return nil
}
