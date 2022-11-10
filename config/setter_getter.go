package config

import (
	"context"
	"time"

	"github.com/ipfs/go-cid"

	"github.com/filecoin-project/go-address"

	"github.com/filecoin-project/go-fil-markets/storagemarket"
	types "github.com/filecoin-project/venus/venus-shared/types/market"
)

// ConsiderOnlineStorageDealsConfigFunc is a function which reads from miner
// config to determine if the miner has disabled storage deals (or not).
type ConsiderOnlineStorageDealsConfigFunc func(address.Address) (bool, error)

// SetConsiderOnlineStorageDealsConfigFunc is a function which is used to
// disable or enable piecestorage deal acceptance.
type SetConsiderOnlineStorageDealsConfigFunc func(address.Address, bool) error

// ConsiderOnlineRetrievalDealsConfigFunc is a function which reads from miner
// config to determine if the user has disabled retrieval acceptance (or not).
type ConsiderOnlineRetrievalDealsConfigFunc func(address.Address) (bool, error)

// SetConsiderOnlineRetrievalDealsConfigFunc is a function which is used to
// disable or enable retrieval deal acceptance.
type SetConsiderOnlineRetrievalDealsConfigFunc func(address.Address, bool) error

// StorageDealPieceCidBlocklistConfigFunc is a function which reads from miner
// config to obtain a list of CIDs for which the miner will not accept
// piecestorage proposals.
type StorageDealPieceCidBlocklistConfigFunc func(address.Address) ([]cid.Cid, error)

// SetStorageDealPieceCidBlocklistConfigFunc is a function which is used to set a
// list of CIDs for which the miner will reject deal proposals.
type SetStorageDealPieceCidBlocklistConfigFunc func(address.Address, []cid.Cid) error

// ConsiderOfflineStorageDealsConfigFunc is a function which reads from miner
// config to determine if the miner has disabled storage deals (or not).
type ConsiderOfflineStorageDealsConfigFunc func(address.Address) (bool, error)

// SetConsiderOfflineStorageDealsConfigFunc is a function which is used to
// disable or enable piecestorage deal acceptance.
type SetConsiderOfflineStorageDealsConfigFunc func(address.Address, bool) error

// ConsiderOfflineRetrievalDealsConfigFunc is a function which reads from miner
// config to determine if the user has disabled retrieval acceptance (or not).
type ConsiderOfflineRetrievalDealsConfigFunc func(address.Address) (bool, error)

// SetConsiderOfflineRetrievalDealsConfigFunc is a function which is used to
// disable or enable retrieval deal acceptance.
type SetConsiderOfflineRetrievalDealsConfigFunc func(address.Address, bool) error

// ConsiderVerifiedStorageDealsConfigFunc is a function which reads from miner
// config to determine if the user has disabled verified piecestorage deals (or not).
type ConsiderVerifiedStorageDealsConfigFunc func(address.Address) (bool, error)

// SetConsiderVerifiedStorageDealsConfigFunc is a function which is used to
// disable or enable verified piecestorage deal acceptance.
type SetConsiderVerifiedStorageDealsConfigFunc func(address.Address, bool) error

// ConsiderUnverifiedStorageDealsConfigFunc is a function which reads from miner
// config to determine if the user has disabled unverified piecestorage deals (or not).
type ConsiderUnverifiedStorageDealsConfigFunc func(address.Address) (bool, error)

// SetConsiderUnverifiedStorageDealsConfigFunc is a function which is used to
// disable or enable unverified piecestorage deal acceptance.
type SetConsiderUnverifiedStorageDealsConfigFunc func(address.Address, bool) error

type (
	SetMaxDealStartDelayFunc func(address.Address, time.Duration) error
	GetMaxDealStartDelayFunc func(address.Address) (time.Duration, error)
)

// SetExpectedSealDurationFunc is a function which is used to set how long sealing is expected to take.
// Deals that would need to start earlier than this duration will be rejected.
type SetExpectedSealDurationFunc func(address.Address, time.Duration) error

// GetExpectedSealDurationFunc is a function which reads from miner
// too determine how long sealing is expected to take
type GetExpectedSealDurationFunc func(address.Address) (time.Duration, error)

type (
	// todo 目前没有被实际使用 ???
	StorageDealFilter   func(ctx context.Context, mAddr address.Address, deal storagemarket.MinerDeal) (bool, string, error)
	// todo 目前没有被实际使用 ???
	RetrievalDealFilter func(ctx context.Context, mAddr address.Address, deal types.ProviderDealState) (bool, string, error)
)
