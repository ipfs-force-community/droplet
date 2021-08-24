package api

import (
	"context"
	"github.com/filecoin-project/go-address"
	datatransfer "github.com/filecoin-project/go-data-transfer"
	"github.com/filecoin-project/go-fil-markets/piecestore"
	"github.com/filecoin-project/go-fil-markets/retrievalmarket"
	"github.com/filecoin-project/go-fil-markets/storagemarket"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/venus-market/types"
	mTypes "github.com/filecoin-project/venus-messager/types"
	vTypes "github.com/filecoin-project/venus/pkg/types"
	"github.com/ipfs/go-cid"
	"github.com/libp2p/go-libp2p-core/peer"
	"golang.org/x/xerrors"
	"time"
)

var x = xerrors.New("") //mock for gen

type MarketNode interface {
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
	MessagerWaitMessage(ctx context.Context, uuid cid.Cid) (*mTypes.Message, error)                      //perm:read
	MessagerPushMessage(ctx context.Context, msg *vTypes.Message, meta *mTypes.MsgMeta) (cid.Cid, error) //perm:write
	MessagerGetMessage(ctx context.Context, uuid cid.Cid) (*mTypes.Message, error)                       //perm:read

}
