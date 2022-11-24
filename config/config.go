package config

import (
	"fmt"

	"github.com/ipfs-force-community/metrics"

	"github.com/filecoin-project/go-address"
)

const (
	// RetrievalPricingDefault configures the node to use the default retrieval pricing policy.
	RetrievalPricingDefaultMode = "default"
	// RetrievalPricingExternal configures the node to use the external retrieval pricing script
	// configured by the user.
	RetrievalPricingExternalMode = "external"
)

type RetrievalPricing struct {
	Strategy string // possible values: "default", "external"

	Default  *RetrievalPricingDefault
	External *RetrievalPricingExternal
}

type RetrievalPricingExternal struct {
	// Path of the external script that will be run to price a retrieval deal.
	// This parameter is ONLY applicable if the retrieval pricing policy strategy has been configured to "external".
	Path string
}

type RetrievalPricingDefault struct {
	// VerifiedDealsFreeTransfer configures zero fees for data transfer for a retrieval deal
	// of a payloadCid that belongs to a verified piecestorage deal.
	// This parameter is ONLY applicable if the retrieval pricing policy strategy has been configured to "default".
	// default value is true
	VerifiedDealsFreeTransfer bool
}

type Journal struct {
	Path string
}

type DAGStoreConfig struct {
	// Path to the dagstore root directory. This directory contains three
	// subdirectories, which can be symlinked to alternative locations if
	// need be:
	//  - ./transients: caches unsealed deals that have been fetched from the
	//    storage subsystem for serving retrievals.
	//  - ./indices: stores shard indices.
	//  - ./datastore: holds the KV store tracking the state of every shard
	//    known to the DAG store.
	// Default value: <LOTUS_MARKETS_PATH>/dagstore (split deployment) or
	// <LOTUS_MINER_PATH>/dagstore (monolith deployment)
	RootDir string

	// The maximum amount of indexing jobs that can run simultaneously.
	// 0 means unlimited.
	// Default value: 5.
	MaxConcurrentIndex int

	// The maximum amount of unsealed deals that can be fetched simultaneously
	// from the storage subsystem. 0 means unlimited.
	// Default value: 0 (unlimited).
	MaxConcurrentReadyFetches int

	// The maximum number of simultaneous inflight API calls to the storage
	// subsystem.
	// Default value: 100.
	MaxConcurrencyStorageCalls int

	// The time between calls to periodic dagstore GC, in time.Duration string
	// representation, e.g. 1m, 5m, 1h.
	// Default value: 1 minute.
	GCInterval Duration

	// MongoTopIndex used to config whether to save top index data to mongo
	MongoTopIndex *MongoTopIndex

	// Transient path used to store temp file for retrieval
	Transient string

	// Index path to store index of piece
	Index string

	// ReadDiretly enable to read piece storage directly skip transient file
	UseTransient bool
}

type MongoTopIndex struct {
	Url string
}

type PieceStorage struct {
	Fs []*FsPieceStorage
	S3 []*S3PieceStorage
}
type FsPieceStorage struct {
	Name     string
	ReadOnly bool
	Path     string
}
type S3PieceStorage struct {
	Name     string
	ReadOnly bool
	EndPoint string
	Bucket   string
	SubDir   string

	AccessKey string
	SecretKey string
	Token     string
}

type Mysql struct {
	ConnectionString string
	MaxOpenConn      int
	MaxIdleConn      int
	ConnMaxLifeTime  string
	Debug            bool
}

type MinerConfig struct {
	Addr    Address
	Account string // todo 在合并run模式后才真正起作用

	*ProviderConfig
}

type MarketConfig struct {
	Home `toml:"-"`

	Common

	// The maximum number of parallel online data transfers for storage deals
	SimultaneousTransfersForStorage uint64
	// The maximum number of simultaneous data transfers from any single client
	// for storage deals.
	// Unset by default (0), and values higher than SimultaneousTransfersForStorage
	// will have no effect; i.e. the total number of simultaneous data transfers
	// across all storage clients is bound by SimultaneousTransfersForStorage
	// regardless of this number.
	SimultaneousTransfersForStoragePerClient uint64
	// The maximum number of parallel online data transfers for retrieval deals
	SimultaneousTransfersForRetrieval uint64

	Node     Node
	Messager Messager
	Signer   Signer
	AuthNode AuthNode

	Mysql Mysql

	PieceStorage PieceStorage
	DAGStore     DAGStoreConfig

	CommonProvider *ProviderConfig
	Miners         []*MinerConfig

	Journal Journal
	Metrics metrics.MetricsConfig
}

func (m *MarketConfig) RemovePieceStorage(name string) error {
	for i, s := range m.PieceStorage.Fs {
		if s.Name == name {
			m.PieceStorage.Fs = append(m.PieceStorage.Fs[:i], m.PieceStorage.Fs[i+1:]...)
			return SaveConfig(m)
		}
	}
	for i, s := range m.PieceStorage.S3 {
		if s.Name == name {
			m.PieceStorage.S3 = append(m.PieceStorage.S3[:i], m.PieceStorage.S3[i+1:]...)
			return SaveConfig(m)
		}
	}
	return fmt.Errorf("piece storage %s not found", name)
}

func (m *MarketConfig) AddFsPieceStorage(fsps *FsPieceStorage) error {
	m.PieceStorage.Fs = append(m.PieceStorage.Fs, fsps)
	return SaveConfig(m)
}

func (m *MarketConfig) AddS3PieceStorage(fsps *S3PieceStorage) error {
	m.PieceStorage.S3 = append(m.PieceStorage.S3, fsps)
	return SaveConfig(m)
}

func (m *MarketConfig) MinerProviderConfig(mAddr address.Address, useCommon bool) *ProviderConfig {
	for i := range m.Miners {
		if m.Miners[i].Addr == Address(mAddr) {
			return m.Miners[i].ProviderConfig
		}
	}

	if useCommon {
		return m.CommonProvider
	}

	return nil
}

func (m *MarketConfig) SetMinerProviderConfig(mAddr address.Address, pCfg *ProviderConfig) {
	if mAddr == address.Undef {
		m.CommonProvider = pCfg
	} else {
		for i := range m.Miners {
			if m.Miners[i].Addr == Address(mAddr) {
				m.Miners[i].ProviderConfig = pCfg
				return
			}
		}

		// create
		m.Miners = append(m.Miners, &MinerConfig{
			Addr:           Address(mAddr),
			ProviderConfig: pCfg,
		})
	}
}
