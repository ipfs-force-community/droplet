package api

import (
	"context"
	"time"

	"github.com/ipfs/go-cid"
	"github.com/libp2p/go-libp2p-core/peer"
	"golang.org/x/xerrors"

	"github.com/filecoin-project/go-address"
	datatransfer "github.com/filecoin-project/go-data-transfer"
	"github.com/filecoin-project/go-fil-markets/piecestore"
	"github.com/filecoin-project/go-fil-markets/retrievalmarket"
	"github.com/filecoin-project/go-fil-markets/storagemarket"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/specs-actors/actors/builtin/paych"

	mTypes "github.com/filecoin-project/venus-messager/types"

	"github.com/ipfs-force-community/venus-gateway/marketevent"
	types2 "github.com/ipfs-force-community/venus-gateway/types"

	"github.com/filecoin-project/venus/app/submodule/apitypes"
	vTypes "github.com/filecoin-project/venus/pkg/types"

	"github.com/filecoin-project/venus-market/client"
	"github.com/filecoin-project/venus-market/imports"
	"github.com/filecoin-project/venus-market/piece"
	"github.com/filecoin-project/venus-market/types"
	"github.com/filecoin-project/venus-market/utils"
)

//mock for gen
var _ = xerrors.New("") // nolint

type MarketFullNode interface {
	ActorAddress(context.Context) ([]address.Address, error)                  //perm:read
	ActorExist(ctx context.Context, addr address.Address) (bool, error)       //perm:read
	ActorSectorSize(context.Context, address.Address) (abi.SectorSize, error) //perm:read

	MarketImportDealData(ctx context.Context, propcid cid.Cid, path string) error                                                                                                                                 //perm:write
	MarketListDeals(ctx context.Context, addrs []address.Address) ([]types.MarketDeal, error)                                                                                                                     //perm:read
	MarketListRetrievalDeals(ctx context.Context, mAddr address.Address) ([]retrievalmarket.ProviderDealState, error)                                                                                             //perm:read
	MarketGetDealUpdates(ctx context.Context) (<-chan storagemarket.MinerDeal, error)                                                                                                                             //perm:read
	MarketListIncompleteDeals(ctx context.Context, mAddr address.Address) ([]storagemarket.MinerDeal, error)                                                                                                      //perm:read
	MarketSetAsk(ctx context.Context, mAddr address.Address, price vTypes.BigInt, verifiedPrice vTypes.BigInt, duration abi.ChainEpoch, minPieceSize abi.PaddedPieceSize, maxPieceSize abi.PaddedPieceSize) error //perm:admin
	MarketGetAsk(ctx context.Context, mAddr address.Address) (*storagemarket.SignedStorageAsk, error)                                                                                                             //perm:read
	MarketSetRetrievalAsk(ctx context.Context, mAddr address.Address, rask *retrievalmarket.Ask) error                                                                                                            //perm:admin
	MarketGetRetrievalAsk(ctx context.Context, mAddr address.Address) (*retrievalmarket.Ask, error)                                                                                                               //perm:read
	MarketListDataTransfers(ctx context.Context) ([]types.DataTransferChannel, error)                                                                                                                             //perm:write
	MarketDataTransferUpdates(ctx context.Context) (<-chan types.DataTransferChannel, error)                                                                                                                      //perm:write
	// MarketRestartDataTransfer attempts to restart a data transfer with the given transfer ID and other peer
	MarketRestartDataTransfer(ctx context.Context, transferID datatransfer.TransferID, otherPeer peer.ID, isInitiator bool) error //perm:write
	// MarketCancelDataTransfer cancels a data transfer with the given transfer ID and other peer
	MarketCancelDataTransfer(ctx context.Context, transferID datatransfer.TransferID, otherPeer peer.ID, isInitiator bool) error //perm:write
	MarketPendingDeals(ctx context.Context) (types.PendingDealInfo, error)                                                       //perm:write
	MarketPublishPendingDeals(ctx context.Context) error                                                                         //perm:admin

	PiecesListPieces(ctx context.Context) ([]cid.Cid, error)                                 //perm:read
	PiecesListCidInfos(ctx context.Context) ([]cid.Cid, error)                               //perm:read
	PiecesGetPieceInfo(ctx context.Context, pieceCid cid.Cid) (*piecestore.PieceInfo, error) //perm:read
	PiecesGetCIDInfo(ctx context.Context, payloadCid cid.Cid) (*piecestore.CIDInfo, error)   //perm:read

	DealsImportData(ctx context.Context, dealPropCid cid.Cid, file string) error //perm:admin
	DealsConsiderOnlineStorageDeals(context.Context) (bool, error)               //perm:admin
	DealsSetConsiderOnlineStorageDeals(context.Context, bool) error              //perm:admin
	DealsConsiderOnlineRetrievalDeals(context.Context) (bool, error)             //perm:admin
	DealsSetConsiderOnlineRetrievalDeals(context.Context, bool) error            //perm:admin
	DealsPieceCidBlocklist(context.Context) ([]cid.Cid, error)                   //perm:admin
	DealsSetPieceCidBlocklist(context.Context, []cid.Cid) error                  //perm:admin
	DealsConsiderOfflineStorageDeals(context.Context) (bool, error)              //perm:admin
	DealsSetConsiderOfflineStorageDeals(context.Context, bool) error             //perm:admin
	DealsConsiderOfflineRetrievalDeals(context.Context) (bool, error)            //perm:admin
	DealsSetConsiderOfflineRetrievalDeals(context.Context, bool) error           //perm:admin
	DealsConsiderVerifiedStorageDeals(context.Context) (bool, error)             //perm:admin
	DealsSetConsiderVerifiedStorageDeals(context.Context, bool) error            //perm:admin
	DealsConsiderUnverifiedStorageDeals(context.Context) (bool, error)           //perm:admin
	DealsSetConsiderUnverifiedStorageDeals(context.Context, bool) error          //perm:admin
	// SectorGetSealDelay gets the time that a newly-created sector
	// waits for more deals before it starts sealing
	SectorGetSealDelay(context.Context) (time.Duration, error) //perm:read
	// SectorSetExpectedSealDuration sets the expected time for a sector to seal
	SectorSetExpectedSealDuration(context.Context, time.Duration) error //perm:write

	//messager
	MessagerWaitMessage(ctx context.Context, mid cid.Cid) (*apitypes.MsgLookup, error)                                 //perm:read
	MessagerPushMessage(ctx context.Context, msg *vTypes.Message, meta *mTypes.MsgMeta) (*vTypes.SignedMessage, error) //perm:write
	MessagerGetMessage(ctx context.Context, mid cid.Cid) (*vTypes.Message, error)                                      //perm:read

	MarketAddBalance(ctx context.Context, wallet, addr address.Address, amt vTypes.BigInt) (cid.Cid, error)                   //perm:sign
	MarketGetReserved(ctx context.Context, addr address.Address) (vTypes.BigInt, error)                                       //perm:sign
	MarketReserveFunds(ctx context.Context, wallet address.Address, addr address.Address, amt vTypes.BigInt) (cid.Cid, error) //perm:sign
	MarketReleaseFunds(ctx context.Context, addr address.Address, amt vTypes.BigInt) error                                    //perm:sign
	MarketWithdraw(ctx context.Context, wallet, addr address.Address, amt vTypes.BigInt) (cid.Cid, error)                     //perm:sign

	NetAddrsListen(context.Context) (peer.AddrInfo, error) //perm:read
	ID(context.Context) (peer.ID, error)                   //perm:read

	// DagstoreListShards returns information about all shards known to the
	// DAG store. Only available on nodes running the markets subsystem.
	DagstoreListShards(ctx context.Context) ([]types.DagstoreShardInfo, error) //perm:read

	// DagstoreInitializeShard initializes an uninitialized shard.
	//
	// Initialization consists of fetching the shard's data (deal payload) from
	// the storage subsystem, generating an index, and persisting the index
	// to facilitate later retrievals, and/or to publish to external sources.
	//
	// This operation is intended to complement the initial migration. The
	// migration registers a shard for every unique piece CID, with lazy
	// initialization. Thus, shards are not initialized immediately to avoid
	// IO activity competing with proving. Instead, shard are initialized
	// when first accessed. This method forces the initialization of a shard by
	// accessing it and immediately releasing it. This is useful to warm up the
	// cache to facilitate subsequent retrievals, and to generate the indexes
	// to publish them externally.
	//
	// This operation fails if the shard is not in ShardStateNew state.
	// It blocks until initialization finishes.
	DagstoreInitializeShard(ctx context.Context, key string) error //perm:write

	// DagstoreRecoverShard attempts to recover a failed shard.
	//
	// This operation fails if the shard is not in ShardStateErrored state.
	// It blocks until recovery finishes. If recovery failed, it returns the
	// error.
	DagstoreRecoverShard(ctx context.Context, key string) error //perm:write

	// DagstoreInitializeAll initializes all uninitialized shards in bulk,
	// according to the policy passed in the parameters.
	//
	// It is recommended to set a maximum concurrency to avoid extreme
	// IO pressure if the storage subsystem has a large amount of deals.
	//
	// It returns a stream of events to report progress.
	DagstoreInitializeAll(ctx context.Context, params types.DagstoreInitializeAllParams) (<-chan types.DagstoreInitializeAllEvent, error) //perm:write

	// DagstoreGC runs garbage collection on the DAG store.
	DagstoreGC(ctx context.Context) ([]types.DagstoreShardResult, error) //perm:admin

	MarkDealsAsPacking(ctx context.Context, miner address.Address, deals []abi.DealID) error                                                             //perm:write
	UpdateDealOnPacking(ctx context.Context, miner address.Address, dealID abi.DealID, sectorid abi.SectorNumber, offset abi.PaddedPieceSize) error      //perm:write
	UpdateDealStatus(ctx context.Context, miner address.Address, dealID abi.DealID, pieceStatus string) error                                            //perm:write
	GetDeals(ctx context.Context, miner address.Address, pageIndex, pageSize int) ([]*piece.DealInfo, error)                                             //perm:read
	GetUnPackedDeals(ctx context.Context, miner address.Address, spec *piece.GetDealSpec) ([]*piece.DealInfoIncludePath, error)                          //perm:read
	AssignUnPackedDeals(ctx context.Context, miner address.Address, ssize abi.SectorSize, spec *piece.GetDealSpec) ([]*piece.DealInfoIncludePath, error) //perm:write

	//market event
	ResponseMarketEvent(ctx context.Context, resp *types2.ResponseEvent) error                                            //perm:read
	ListenMarketEvent(ctx context.Context, policy *marketevent.MarketRegisterPolicy) (<-chan *types2.RequestEvent, error) //perm:read

	// Paych
	PaychVoucherList(ctx context.Context, pch address.Address) ([]*paych.SignedVoucher, error) //perm:read
}

type MarketClientNode interface {

	// ClientImport imports file under the specified path into filestore.
	ClientImport(ctx context.Context, ref client.FileRef) (*client.ImportRes, error) //perm:admin
	// ClientRemoveImport removes file import
	ClientRemoveImport(ctx context.Context, importID imports.ID) error //perm:admin
	// ClientStartDeal proposes a deal with a miner.
	ClientStartDeal(ctx context.Context, params *client.StartDealParams) (*cid.Cid, error) //perm:admin
	// ClientStatelessDeal fire-and-forget-proposes an offline deal to a miner without subsequent tracking.
	ClientStatelessDeal(ctx context.Context, params *client.StartDealParams) (*cid.Cid, error) //perm:write
	// ClientGetDealInfo returns the latest information about a given deal.
	ClientGetDealInfo(context.Context, cid.Cid) (*client.DealInfo, error) //perm:read
	// ClientListDeals returns information about the deals made by the local client.
	ClientListDeals(ctx context.Context) ([]client.DealInfo, error) //perm:write
	// ClientGetDealUpdates returns the status of updated deals
	ClientGetDealUpdates(ctx context.Context) (<-chan client.DealInfo, error) //perm:write
	// ClientGetDealStatus returns status given a code
	ClientGetDealStatus(ctx context.Context, statusCode uint64) (string, error) //perm:read
	// ClientHasLocal indicates whether a certain CID is locally stored.
	ClientHasLocal(ctx context.Context, root cid.Cid) (bool, error) //perm:write
	// ClientFindData identifies peers that have a certain file, and returns QueryOffers (one per peer).
	ClientFindData(ctx context.Context, root cid.Cid, piece *cid.Cid) ([]client.QueryOffer, error) //perm:read
	// ClientMinerQueryOffer returns a QueryOffer for the specific miner and file.
	ClientMinerQueryOffer(ctx context.Context, miner address.Address, root cid.Cid, piece *cid.Cid) (client.QueryOffer, error) //perm:read
	// ClientRetrieve initiates the retrieval of a file, as specified in the order.
	ClientRetrieve(ctx context.Context, order client.RetrievalOrder, ref *client.FileRef) error //perm:admin
	// ClientRetrieveWithEvents initiates the retrieval of a file, as specified in the order, and provides a channel
	// of status updates.
	ClientRetrieveWithEvents(ctx context.Context, order client.RetrievalOrder, ref *client.FileRef) (<-chan utils.RetrievalEvent, error) //perm:admin
	// ClientListRetrievals returns information about retrievals made by the local client
	ClientListRetrievals(ctx context.Context) ([]client.RetrievalInfo, error) //perm:write
	// ClientGetRetrievalUpdates returns status of updated retrieval deals
	ClientGetRetrievalUpdates(ctx context.Context) (<-chan client.RetrievalInfo, error) //perm:write
	// ClientQueryAsk returns a signed StorageAsk from the specified miner.
	ClientQueryAsk(ctx context.Context, p peer.ID, miner address.Address) (*storagemarket.StorageAsk, error) //perm:read
	// ClientCalcCommP calculates the CommP and data size of the specified CID
	ClientDealPieceCID(ctx context.Context, root cid.Cid) (client.DataCIDSize, error) //perm:read
	// ClientCalcCommP calculates the CommP for a specified file
	ClientCalcCommP(ctx context.Context, inpath string) (*client.CommPRet, error) //perm:write
	// ClientGenCar generates a CAR file for the specified file.
	ClientGenCar(ctx context.Context, ref client.FileRef, outpath string) error //perm:write
	// ClientDealSize calculates real deal data size
	ClientDealSize(ctx context.Context, root cid.Cid) (client.DataSize, error) //perm:read
	// ClientListTransfers returns the status of all ongoing transfers of data
	ClientListDataTransfers(ctx context.Context) ([]types.DataTransferChannel, error)        //perm:write
	ClientDataTransferUpdates(ctx context.Context) (<-chan types.DataTransferChannel, error) //perm:write
	// ClientRestartDataTransfer attempts to restart a data transfer with the given transfer ID and other peer
	ClientRestartDataTransfer(ctx context.Context, transferID datatransfer.TransferID, otherPeer peer.ID, isInitiator bool) error //perm:write
	// ClientCancelDataTransfer cancels a data transfer with the given transfer ID and other peer
	ClientCancelDataTransfer(ctx context.Context, transferID datatransfer.TransferID, otherPeer peer.ID, isInitiator bool) error //perm:write
	// ClientRetrieveTryRestartInsufficientFunds attempts to restart stalled retrievals on a given payment channel
	// which are stuck due to insufficient funds
	ClientRetrieveTryRestartInsufficientFunds(ctx context.Context, paymentChannel address.Address) error //perm:write

	// ClientCancelRetrievalDeal cancels an ongoing retrieval deal based on DealID
	ClientCancelRetrievalDeal(ctx context.Context, dealid retrievalmarket.DealID) error //perm:write

	// ClientUnimport removes references to the specified file from filestore
	//ClientUnimport(path string)

	// ClientListImports lists imported files and their root CIDs
	ClientListImports(ctx context.Context) ([]client.Import, error) //perm:write
	DefaultAddress(ctx context.Context) (address.Address, error)    //perm:read

	MarketAddBalance(ctx context.Context, wallet, addr address.Address, amt vTypes.BigInt) (cid.Cid, error)                   //perm:write
	MarketGetReserved(ctx context.Context, addr address.Address) (vTypes.BigInt, error)                                       //perm:read
	MarketReserveFunds(ctx context.Context, wallet address.Address, addr address.Address, amt vTypes.BigInt) (cid.Cid, error) //perm:write
	MarketReleaseFunds(ctx context.Context, addr address.Address, amt vTypes.BigInt) error                                    //perm:write
	MarketWithdraw(ctx context.Context, wallet, addr address.Address, amt vTypes.BigInt) (cid.Cid, error)                     //perm:write
}
