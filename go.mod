module github.com/filecoin-project/venus-market/v2

go 1.16

require (
	github.com/BurntSushi/toml v0.4.1
	github.com/acarl005/stripansi v0.0.0-20180116102854-5a71ef0e047d
	github.com/aws/aws-sdk-go v1.43.10
	github.com/buger/goterm v0.0.0-20200322175922-2f3e71b85129
	github.com/chzyer/readline v0.0.0-20180603132655-2972be24d48e
	github.com/dgraph-io/badger/v2 v2.2007.3
	github.com/docker/go-units v0.4.0
	github.com/fatih/color v1.13.0
	github.com/filecoin-project/dagstore v0.5.2
	github.com/filecoin-project/go-address v0.0.6
	github.com/filecoin-project/go-bitfield v0.2.4
	github.com/filecoin-project/go-cbor-util v0.0.1
	github.com/filecoin-project/go-commp-utils v0.1.3
	github.com/filecoin-project/go-data-transfer v1.15.1
	github.com/filecoin-project/go-fil-commcid v0.1.0
	github.com/filecoin-project/go-fil-commp-hashhash v0.1.0
	github.com/filecoin-project/go-fil-markets v1.20.1
	github.com/filecoin-project/go-jsonrpc v0.1.4-0.20210217175800-45ea43ac2bec
	github.com/filecoin-project/go-padreader v0.0.1
	github.com/filecoin-project/go-state-types v0.1.3
	github.com/filecoin-project/go-statemachine v1.0.2-0.20220322104818-27f8fbb86dfd
	github.com/filecoin-project/go-statestore v0.2.0
	github.com/filecoin-project/specs-actors v0.9.14
	github.com/filecoin-project/specs-actors/v2 v2.3.6
	github.com/filecoin-project/specs-actors/v7 v7.0.0
	github.com/filecoin-project/specs-storage v0.2.2
	github.com/filecoin-project/venus v1.2.4-0.20220420072943-4d565663fa60
	github.com/filecoin-project/venus-auth v1.3.3-0.20220406063133-896f44f6e816
	github.com/filecoin-project/venus-messager v1.2.2-rc1.0.20220420091920-4820c01ca309
	github.com/gbrlsnchs/jwt/v3 v3.0.1
	github.com/golang/mock v1.6.0
	github.com/google/uuid v1.3.0
	github.com/gorilla/mux v1.8.0
	github.com/hannahhoward/go-pubsub v0.0.0-20200423002714-8d62886cc36e
	github.com/ipfs-force-community/venus-common-utils v0.0.0-20211122032945-eb6cab79c62a
	github.com/ipfs-force-community/venus-gateway v1.2.1-0.20220420085343-2e500a7724e0
	github.com/ipfs/go-block-format v0.0.3
	github.com/ipfs/go-blockservice v0.2.1
	github.com/ipfs/go-cid v0.1.0
	github.com/ipfs/go-cidutil v0.0.2
	github.com/ipfs/go-datastore v0.5.1
	github.com/ipfs/go-ds-badger2 v0.1.2
	github.com/ipfs/go-ds-leveldb v0.5.0
	github.com/ipfs/go-ds-measure v0.2.0
	github.com/ipfs/go-graphsync v0.13.1
	github.com/ipfs/go-ipfs-blockstore v1.1.2
	github.com/ipfs/go-ipfs-blocksutil v0.0.1
	github.com/ipfs/go-ipfs-chunker v0.0.5
	github.com/ipfs/go-ipfs-exchange-offline v0.1.1
	github.com/ipfs/go-ipfs-files v0.0.9
	github.com/ipfs/go-ipfs-util v0.0.2
	github.com/ipfs/go-ipld-cbor v0.0.6
	github.com/ipfs/go-ipld-format v0.2.0
	github.com/ipfs/go-log/v2 v2.5.1
	github.com/ipfs/go-merkledag v0.5.1
	github.com/ipfs/go-metrics-interface v0.0.1
	github.com/ipfs/go-unixfs v0.3.1
	github.com/ipld/go-car v0.3.3
	github.com/ipld/go-car/v2 v2.1.1
	github.com/ipld/go-codec-dagpb v1.3.1
	github.com/ipld/go-ipld-prime v0.16.0
	github.com/ipld/go-ipld-selector-text-lite v0.0.1
	github.com/libp2p/go-buffer-pool v0.0.2
	github.com/libp2p/go-libp2p v0.19.1
	github.com/libp2p/go-libp2p-core v0.15.1
	github.com/libp2p/go-libp2p-noise v0.4.0
	github.com/libp2p/go-libp2p-peerstore v0.6.0
	github.com/libp2p/go-libp2p-quic-transport v0.17.0
	github.com/libp2p/go-libp2p-resource-manager v0.2.1
	github.com/libp2p/go-libp2p-tls v0.4.1
	github.com/libp2p/go-libp2p-yamux v0.9.1
	github.com/libp2p/go-maddr-filter v0.1.0
	github.com/mitchellh/go-homedir v1.1.0
	github.com/multiformats/go-base32 v0.0.4
	github.com/multiformats/go-multiaddr v0.5.0
	github.com/multiformats/go-multibase v0.0.3
	github.com/multiformats/go-multihash v0.1.0
	github.com/multiformats/go-varint v0.0.6
	github.com/pkg/errors v0.9.1
	github.com/stretchr/testify v1.7.0
	github.com/strikesecurity/strikememongo v0.2.4
	github.com/syndtr/goleveldb v1.0.0
	github.com/urfave/cli/v2 v2.3.0
	github.com/whyrusleeping/cbor-gen v0.0.0-20220302191723-37c43cae8e14
	github.com/whyrusleeping/multiaddr-filter v0.0.0-20160516205228-e903e4adabd7
	github.com/xlab/c-for-go v0.0.0-20201223145653-3ba5db515dcb // indirect
	go.mongodb.org/mongo-driver v1.8.4
	go.opencensus.io v0.23.0
	go.uber.org/fx v1.15.0
	go.uber.org/multierr v1.8.0
	go.uber.org/zap v1.21.0
	golang.org/x/sync v0.0.0-20210220032951-036812b2e83c
	gorm.io/driver/mysql v1.1.1
	gorm.io/gorm v1.21.12
)

replace (
	github.com/filecoin-project/filecoin-ffi => ./extern/filecoin-ffi
	github.com/filecoin-project/go-commp-utils => github.com/ipfs-force-community/go-commp-utils v0.1.4-0.20220429021603-dcbcb96e4fc7
	github.com/filecoin-project/go-fil-markets => github.com/hunjixin/go-fil-markets v1.13.3-0.20220511024045-d61f9911bade
	github.com/filecoin-project/go-jsonrpc => github.com/ipfs-force-community/go-jsonrpc v0.1.4-0.20210721095535-a67dff16de21
)
