package config

import (
	"fmt"
	"net/url"

	"github.com/ipfs-force-community/metrics"
	"github.com/multiformats/go-multiaddr"
	maNet "github.com/multiformats/go-multiaddr/net"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/venus/venus-shared/types"
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
	// Default value: 0, disabled GC.
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
	Account string

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

	ChainService *ChainService

	Node     *Node
	Messager *Messager
	AuthNode *AuthNode
	Signer   Signer

	Mysql Mysql

	PieceStorage PieceStorage
	DAGStore     DAGStoreConfig

	CommonProvider *ProviderConfig
	Miners         []*MinerConfig

	Journal Journal
	Metrics metrics.MetricsConfig
}

func (m *MarketConfig) GetNode() Node {
	ret := Node{}
	if m.Node != nil {
		ret = *m.Node
	}
	chainService := ChainService{}
	if m.ChainService != nil {
		chainService = *m.ChainService
	}
	if ret.Url == "" && chainService.Url != "" {
		ret.Url = chainService.Url
	}
	if ret.Token == "" && chainService.Token != "" {
		ret.Token = chainService.Token
	}
	return ret
}

func (m *MarketConfig) GetMessager() Messager {
	ret := Messager{}
	if m.Messager != nil {
		ret = *m.Messager
	}
	chainService := ChainService{}
	if m.ChainService != nil {
		chainService = *m.ChainService
	}
	if ret.Url == "" && chainService.Url != "" {
		ret.Url = chainService.Url
	}
	if ret.Token == "" && chainService.Token != "" {
		ret.Token = chainService.Token
	}
	return ret
}

func (m *MarketConfig) GetAuthNode() AuthNode {
	if m.Signer.SignerType == SignerTypeWallet {
		return AuthNode{}
	}

	ret := AuthNode{}
	if m.AuthNode != nil {
		ret = *m.AuthNode
	}
	chainService := ChainService{}
	if m.ChainService != nil {
		chainService = *m.ChainService
	}
	if ret.Url == "" && chainService.Url != "" {
		// transfer chainService.Url to AuthNode.Url
		u, err := ParseAddr(chainService.Url)
		if err != nil {
			panic(fmt.Errorf("parse chainService.Url %s fail %w", chainService.Url, err))
		}
		ret.Url = u
	}

	if ret.Token == "" && chainService.Token != "" {
		ret.Token = chainService.Token
	}
	return ret
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

// MinerProviderConfig returns provider config. if mAddr is empty, returns global provider config.
func (m *MarketConfig) MinerProviderConfig(mAddr address.Address, useCommon bool) (*ProviderConfig, error) {
	if mAddr.Empty() {
		return m.CommonProvider, nil
	}

	var found bool
	var minerCfg *ProviderConfig
	for i := range m.Miners {
		if m.Miners[i].Addr == Address(mAddr) {
			found = true
			minerCfg = m.Miners[i].ProviderConfig
			break
		}
	}
	if !found {
		return nil, fmt.Errorf("not found miner(%s) config", mAddr)
	}

	if minerCfg == nil {
		if useCommon {
			return m.CommonProvider, nil
		}

		return minerCfg, nil
	}

	// minerCfg not nil
	if !useCommon {
		return minerCfg, nil
	}

	mergeProviderConfig(minerCfg, m.CommonProvider)

	return minerCfg, nil
}

func mergeProviderConfig(providerCfg, commonCfg *ProviderConfig) {
	nilOrZero := func(val types.FIL) bool {
		return val.Int == nil || val.Int.Cmp(big.Zero().Int) == 0
	}

	if len(providerCfg.PieceCidBlocklist) == 0 && len(commonCfg.PieceCidBlocklist) != 0 {
		providerCfg.PieceCidBlocklist = commonCfg.PieceCidBlocklist
	}
	if providerCfg.ExpectedSealDuration == 0 && commonCfg.ExpectedSealDuration != 0 {
		providerCfg.ExpectedSealDuration = commonCfg.ExpectedSealDuration
	}
	if providerCfg.MaxDealStartDelay == 0 && commonCfg.MaxDealStartDelay != 0 {
		providerCfg.MaxDealStartDelay = commonCfg.MaxDealStartDelay
	}
	if providerCfg.PublishMsgPeriod == 0 && commonCfg.PublishMsgPeriod != 0 {
		providerCfg.PublishMsgPeriod = commonCfg.PublishMsgPeriod
	}
	if providerCfg.MaxDealsPerPublishMsg == 0 && commonCfg.MaxDealsPerPublishMsg != 0 {
		providerCfg.MaxDealsPerPublishMsg = commonCfg.MaxDealsPerPublishMsg
	}
	if len(providerCfg.Filter) == 0 && len(commonCfg.Filter) != 0 {
		providerCfg.Filter = commonCfg.Filter
	}
	if len(providerCfg.RetrievalFilter) == 0 && len(commonCfg.RetrievalFilter) != 0 {
		providerCfg.RetrievalFilter = commonCfg.RetrievalFilter
	}
	if len(providerCfg.TransferPath) == 0 && len(commonCfg.TransferPath) != 0 {
		providerCfg.TransferPath = commonCfg.TransferPath
	}
	if providerCfg.RetrievalPricing == nil && commonCfg.RetrievalPricing != nil {
		providerCfg.RetrievalFilter = commonCfg.RetrievalFilter
	}
	if nilOrZero(providerCfg.MaxPublishDealsFee) && !nilOrZero(commonCfg.MaxPublishDealsFee) {
		providerCfg.MaxPublishDealsFee.Int = commonCfg.MaxPublishDealsFee.Int
	}
	if nilOrZero(providerCfg.MaxMarketBalanceAddFee) && !nilOrZero(commonCfg.MaxMarketBalanceAddFee) {
		providerCfg.MaxMarketBalanceAddFee.Int = commonCfg.MaxMarketBalanceAddFee.Int
	}
	if address.Address(providerCfg.RetrievalPaymentAddress).Empty() {
		providerCfg.RetrievalPaymentAddress = commonCfg.RetrievalPaymentAddress
	}
	if len(providerCfg.DealPublishAddress) == 0 && len(commonCfg.DealPublishAddress) != 0 {
		providerCfg.DealPublishAddress = commonCfg.DealPublishAddress
	}
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
	}
}

// ParseAddr parse a multi addr to a traditional url ( with http scheme as default)
func ParseAddr(addr string) (string, error) {
	ret := addr
	ma, err := multiaddr.NewMultiaddr(addr)
	if err == nil {
		_, addr, err := maNet.DialArgs(ma)
		if err != nil {
			return "", fmt.Errorf("parser libp2p url fail %w", err)
		}

		ret = "http://" + addr

		_, err = ma.ValueForProtocol(multiaddr.P_WSS)
		if err == nil {
			ret = "wss://" + addr
		} else if err != multiaddr.ErrProtocolNotFound {
			return "", err
		}

		_, err = ma.ValueForProtocol(multiaddr.P_HTTPS)
		if err == nil {
			ret = "https://" + addr
		} else if err != multiaddr.ErrProtocolNotFound {
			return "", err
		}

		_, err = ma.ValueForProtocol(multiaddr.P_WS)
		if err == nil {
			ret = "ws://" + addr
		} else if err != multiaddr.ErrProtocolNotFound {
			return "", err
		}

		_, err = ma.ValueForProtocol(multiaddr.P_HTTP)
		if err == nil {
			ret = "http://" + addr
		} else if err != multiaddr.ErrProtocolNotFound {
			return "", err
		}
	}

	_, err = url.Parse(ret)
	if err != nil {
		return "", fmt.Errorf("parser address fail %w", err)
	}

	return ret, nil
}
