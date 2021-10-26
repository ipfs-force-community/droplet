package storageadapter

import (
	datatransfer "github.com/filecoin-project/go-data-transfer"
	"github.com/filecoin-project/go-fil-markets/filestore"
	"github.com/filecoin-project/go-fil-markets/piecestore"
	"github.com/filecoin-project/go-fil-markets/storagemarket/impl/connmanager"
	"github.com/filecoin-project/go-fil-markets/storagemarket/impl/dtutils"
	"github.com/filecoin-project/go-fil-markets/storagemarket/impl/requestvalidation"
	"github.com/filecoin-project/go-fil-markets/storagemarket/network"
	"github.com/filecoin-project/go-fil-markets/stores"
	"github.com/filecoin-project/venus/app/client/apiface"
	cbg "github.com/whyrusleeping/cbor-gen"
	"time"
)

type StorageProviderV2 struct {
	apiFull apiface.FullNode
	net     network.StorageMarketNetwork
	fs      filestore.FileStore
	spn     StorageProviderNode

	pieceStore   piecestore.PieceStore
	conns        *connmanager.ConnManager
	storedAsk    StorageAsk
	dataTransfer datatransfer.Manager

	deals StorageDealStore

	unsubDataTransfer datatransfer.Unsubscribe

	dagStore stores.DAGStoreWrapper

	transferProcess TransferProcess
}

// NewProvider returns a new storage provider
func NewProvider(net network.StorageMarketNetwork,
	fs filestore.FileStore,
	dagStore stores.DAGStoreWrapper,
	pieceStore piecestore.PieceStore,
	dataTransfer datatransfer.Manager,
	spn StorageProviderNode,
	storedAsk StorageAsk,
) (*StorageProviderV2, error) {
	h := &StorageProviderV2{
		net:          net,
		spn:          spn,
		fs:           fs,
		pieceStore:   pieceStore,
		conns:        connmanager.NewConnManager(),
		storedAsk:    storedAsk,
		dataTransfer: dataTransfer,
		dagStore:     dagStore,
	}

	// register a data transfer event handler -- this will send events to the state machines based on DT events
	h.unsubDataTransfer = dataTransfer.SubscribeToEvents(ProviderDataTransferSubscriber(h.transferProcess)) //

	err := dataTransfer.RegisterVoucherType(&requestvalidation.StorageDataTransferVoucher{}, requestvalidation.NewUnifiedRequestValidator(&providerPushDeals{h.deals}, nil))
	if err != nil {
		return nil, err
	}

	err = dataTransfer.RegisterTransportConfigurer(&requestvalidation.StorageDataTransferVoucher{}, dtutils.TransportConfigurer(newProviderStoreGetter(h.deals)))
	if err != nil {
		return nil, err
	}

	return h, nil
}

func curTime() cbg.CborTime {
	now := time.Now()
	return cbg.CborTime(time.Unix(0, now.UnixNano()).UTC())
}
