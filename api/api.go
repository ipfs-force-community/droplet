package api

import (
	"context"
	"github.com/filecoin-project/go-address"
	datatransfer "github.com/filecoin-project/go-data-transfer"
	"github.com/filecoin-project/go-fil-markets/piecestore"
	"github.com/filecoin-project/go-fil-markets/retrievalmarket"
	"github.com/filecoin-project/go-fil-markets/storagemarket"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/venus-market/client"
	"github.com/filecoin-project/venus-market/imports"
	"github.com/filecoin-project/venus-market/piece"
	"github.com/filecoin-project/venus-market/types"
	"github.com/filecoin-project/venus-market/utils"
	mTypes "github.com/filecoin-project/venus-messager/types"
	vTypes "github.com/filecoin-project/venus/pkg/types"
	"github.com/google/uuid"
	"github.com/ipfs/go-cid"
	"github.com/libp2p/go-libp2p-core/peer"
	"golang.org/x/xerrors"
	"time"
)

var x = xerrors.New("") //mock for gen

type MarketFullNode interface {
	ActorAddress(context.Context) (address.Address, error)                    //perm:read
	ActorSectorSize(context.Context, address.Address) (abi.SectorSize, error) //perm:read

	MarketImportDealData(ctx context.Context, propcid cid.Cid, path string) error                                                                                                          //perm:write
	MarketListDeals(ctx context.Context) ([]types.MarketDeal, error)                                                                                                                       //perm:read
	MarketListRetrievalDeals(ctx context.Context) ([]retrievalmarket.ProviderDealState, error)                                                                                             //perm:read
	MarketGetDealUpdates(ctx context.Context) (<-chan storagemarket.MinerDeal, error)                                                                                                      //perm:read
	MarketListIncompleteDeals(ctx context.Context) ([]storagemarket.MinerDeal, error)                                                                                                      //perm:read
	MarketSetAsk(ctx context.Context, price vTypes.BigInt, verifiedPrice vTypes.BigInt, duration abi.ChainEpoch, minPieceSize abi.PaddedPieceSize, maxPieceSize abi.PaddedPieceSize) error //perm:admin
	MarketGetAsk(ctx context.Context) (*storagemarket.SignedStorageAsk, error)                                                                                                             //perm:read
	MarketSetRetrievalAsk(ctx context.Context, rask *retrievalmarket.Ask) error                                                                                                            //perm:admin
	MarketGetRetrievalAsk(ctx context.Context) (*retrievalmarket.Ask, error)                                                                                                               //perm:read
	MarketListDataTransfers(ctx context.Context) ([]types.DataTransferChannel, error)                                                                                                      //perm:write
	MarketDataTransferUpdates(ctx context.Context) (<-chan types.DataTransferChannel, error)                                                                                               //perm:write
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
	DealsList(ctx context.Context) ([]types.MarketDeal, error)                   //perm:admin
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
	MessagerWaitMessage(ctx context.Context, uuid uuid.UUID) (*mTypes.Message, error)                      //perm:read
	MessagerPushMessage(ctx context.Context, msg *vTypes.Message, meta *mTypes.MsgMeta) (uuid.UUID, error) //perm:write
	MessagerGetMessage(ctx context.Context, uuid uuid.UUID) (*mTypes.Message, error)                       //perm:read

	MarketAddBalance(ctx context.Context, wallet, addr address.Address, amt vTypes.BigInt) (cid.Cid, error)                   //perm:sign
	MarketGetReserved(ctx context.Context, addr address.Address) (vTypes.BigInt, error)                                       //perm:sign
	MarketReserveFunds(ctx context.Context, wallet address.Address, addr address.Address, amt vTypes.BigInt) (cid.Cid, error) //perm:sign
	MarketReleaseFunds(ctx context.Context, addr address.Address, amt vTypes.BigInt) error                                    //perm:sign
	MarketWithdraw(ctx context.Context, wallet, addr address.Address, amt vTypes.BigInt) (cid.Cid, error)                     //perm:sign

	NetAddrsListen(context.Context) (peer.AddrInfo, error) //perm:read
	ID(context.Context) (peer.ID, error)                   //perm:read

	//todo validate miner identify
	GetDeals(miner address.Address, pageIndex, pageSize int) ([]*piece.DealInfo, error)                                                          //perm:read
	GetUnPackedDeals(miner address.Address, spec *piece.GetDealSpec) ([]*piece.DealInfo, error)                                                  //perm:read
	MarkDealsAsPacking(miner address.Address, deals []abi.DealID) error                                                                          //perm:write
	UpdateDealOnPacking(miner address.Address, pieceCID cid.Cid, dealId abi.DealID, sectorid abi.SectorNumber, offset abi.PaddedPieceSize) error //perm:write
	UpdateDealStatus(miner address.Address, dealId abi.DealID, status string) error                                                              //perm:write
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

	MarketAddBalance(ctx context.Context, wallet, addr address.Address, amt vTypes.BigInt) (cid.Cid, error)
	MarketGetReserved(ctx context.Context, addr address.Address) (vTypes.BigInt, error)
	MarketReserveFunds(ctx context.Context, wallet address.Address, addr address.Address, amt vTypes.BigInt) (cid.Cid, error)
	MarketReleaseFunds(ctx context.Context, addr address.Address, amt vTypes.BigInt) error
	MarketWithdraw(ctx context.Context, wallet, addr address.Address, amt vTypes.BigInt) (cid.Cid, error)
}
