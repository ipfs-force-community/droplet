module github.com/filecoin-project/venus-market

go 1.16

require (
	github.com/dgraph-io/badger v1.6.1
	github.com/dgraph-io/badger/v2 v2.2007.2
	github.com/filecoin-project/go-address v0.0.5
	github.com/filecoin-project/go-fil-markets v1.2.4
	github.com/filecoin-project/go-jsonrpc v0.1.4-0.20210217175800-45ea43ac2bec
	github.com/filecoin-project/go-multistore v0.0.3
	github.com/filecoin-project/venus v0.9.6
	github.com/filecoin-project/venus-messager v1.0.5
	github.com/ipfs/go-block-format v0.0.3
	github.com/ipfs/go-cid v0.0.7
	github.com/ipfs/go-datastore v0.4.5
	github.com/ipfs/go-ipfs-blockstore v1.0.3
	github.com/ipfs/go-log/v2 v2.1.3
	github.com/libp2p/go-buffer-pool v0.0.2
	github.com/libp2p/go-libp2p-core v0.8.5 // indirect
	github.com/multiformats/go-base32 v0.0.3
	github.com/multiformats/go-multiaddr v0.3.1
	github.com/pelletier/go-toml v1.9.0
	github.com/urfave/cli/v2 v2.3.0
	go.uber.org/fx v1.13.1
	go.uber.org/zap v1.16.0
	golang.org/x/xerrors v0.0.0-20200804184101-5ec99f83aff1
	gorm.io/driver/mysql v1.0.5
	gorm.io/driver/sqlite v1.1.4
	gorm.io/gorm v1.21.3
)

replace github.com/filecoin-project/filecoin-ffi => ./extern/filecoin-ffi
