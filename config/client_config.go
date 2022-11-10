package config

type MarketClientConfig struct {
	Home `toml:"-"`
	Common

	Node     Node
	Messager Messager
	Signer   Signer

	// The maximum number of parallel online data transfers (piecestorage+retrieval)
	SimultaneousTransfersForRetrieval uint64
	SimultaneousTransfersForStorage   uint64
	DefaultMarketAddress              Address
}
