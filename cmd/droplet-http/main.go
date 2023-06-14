package main

import (
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/etherlabsio/healthcheck/v2"
	"github.com/ipfs-force-community/droplet/v2/utils"
	"github.com/ipfs-force-community/droplet/v2/version"
	"github.com/urfave/cli/v2"
)

func main() {
	app := cli.App{
		Name:    "droplet http",
		Usage:   "retrieve deal data by http",
		Version: version.UserVersion(),
		Flags:   []cli.Flag{},
		Commands: []*cli.Command{
			{
				Name:  "run",
				Usage: "run http server",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:  "listen",
						Value: "127.0.0.1:53241",
					},
					dropletRepoFlag,
				},
				Action: run,
			},
			queryProtocols,
		},
	}

	if err := app.Run(os.Args); err != nil {
		fmt.Println(err)
	}
}

func run(cctx *cli.Context) error {
	utils.SetupLogLevels()

	ctx := cctx.Context
	listen := cctx.String("listen")
	dropletRepoPath := cctx.String("droplet-repo")

	ser, err := newServer(dropletRepoPath)
	if err != nil {
		return err
	}
	mux := http.DefaultServeMux
	mux.Handle("/healthcheck", healthcheck.Handler())
	mux.HandleFunc("/Version", ser.Version)
	mux.HandleFunc("/piece/", ser.retrievalByPieceCID)

	server := &http.Server{
		Addr:    listen,
		Handler: mux,
	}
	fmt.Println("listen:", listen)
	go func() {
		if err := server.ListenAndServe(); err != nil {
			fmt.Printf("start serve failed: %v\n", err)
		}
	}()

	sig := make(chan os.Signal, 2)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig

	fmt.Println("shutdown server")
	if err := server.Shutdown(ctx); err != nil {
		return err
	}
	fmt.Println("server exited")

	return nil
}
