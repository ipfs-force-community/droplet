package config

import (
	"context"
	"time"

	"github.com/filecoin-project/go-fil-markets/storagemarket"
	types "github.com/filecoin-project/venus/venus-shared/types/market"
	"github.com/ipfs/go-cid"
)

// ConsiderOnlineStorageDealsConfigFunc is a function which reads from miner
// config to determine if the user has disabled piecestorage deals (or not).
type ConsiderOnlineStorageDealsConfigFunc func() (bool, error)

// SetConsiderOnlineStorageDealsConfigFunc is a function which is used to
// disable or enable piecestorage deal acceptance.
type SetConsiderOnlineStorageDealsConfigFunc func(bool) error

// ConsiderOnlineRetrievalDealsConfigFunc is a function which reads from miner
// config to determine if the user has disabled retrieval acceptance (or not).
type ConsiderOnlineRetrievalDealsConfigFunc func() (bool, error)

// SetConsiderOnlineRetrievalDealsConfigFunc is a function which is used to
// disable or enable retrieval deal acceptance.
type SetConsiderOnlineRetrievalDealsConfigFunc func(bool) error

// StorageDealPieceCidBlocklistConfigFunc is a function which reads from miner
// config to obtain a list of CIDs for which the miner will not accept
// piecestorage proposals.
type StorageDealPieceCidBlocklistConfigFunc func() ([]cid.Cid, error)

// SetStorageDealPieceCidBlocklistConfigFunc is a function which is used to set a
// list of CIDs for which the miner will reject deal proposals.
type SetStorageDealPieceCidBlocklistConfigFunc func([]cid.Cid) error

// ConsiderOfflineStorageDealsConfigFunc is a function which reads from miner
// config to determine if the user has disabled piecestorage deals (or not).
type ConsiderOfflineStorageDealsConfigFunc func() (bool, error)

// SetConsiderOfflineStorageDealsConfigFunc is a function which is used to
// disable or enable piecestorage deal acceptance.
type SetConsiderOfflineStorageDealsConfigFunc func(bool) error

// ConsiderOfflineRetrievalDealsConfigFunc is a function which reads from miner
// config to determine if the user has disabled retrieval acceptance (or not).
type ConsiderOfflineRetrievalDealsConfigFunc func() (bool, error)

// SetConsiderOfflineRetrievalDealsConfigFunc is a function which is used to
// disable or enable retrieval deal acceptance.
type SetConsiderOfflineRetrievalDealsConfigFunc func(bool) error

// ConsiderVerifiedStorageDealsConfigFunc is a function which reads from miner
// config to determine if the user has disabled verified piecestorage deals (or not).
type ConsiderVerifiedStorageDealsConfigFunc func() (bool, error)

// SetConsiderVerifiedStorageDealsConfigFunc is a function which is used to
// disable or enable verified piecestorage deal acceptance.
type SetConsiderVerifiedStorageDealsConfigFunc func(bool) error

// ConsiderUnverifiedStorageDealsConfigFunc is a function which reads from miner
// config to determine if the user has disabled unverified piecestorage deals (or not).
type ConsiderUnverifiedStorageDealsConfigFunc func() (bool, error)

// SetConsiderUnverifiedStorageDealsConfigFunc is a function which is used to
// disable or enable unverified piecestorage deal acceptance.
type SetConsiderUnverifiedStorageDealsConfigFunc func(bool) error

type SetMaxDealStartDelayFunc func(time.Duration) error
type GetMaxDealStartDelayFunc func() (time.Duration, error)

// SetExpectedSealDurationFunc is a function which is used to set how long sealing is expected to take.
// Deals that would need to start earlier than this duration will be rejected.
type SetExpectedSealDurationFunc func(time.Duration) error

// GetExpectedSealDurationFunc is a function which reads from miner
// too determine how long sealing is expected to take
type GetExpectedSealDurationFunc func() (time.Duration, error)

type StorageDealFilter func(ctx context.Context, deal storagemarket.MinerDeal) (bool, string, error)
type RetrievalDealFilter func(ctx context.Context, deal types.ProviderDealState) (bool, string, error)
