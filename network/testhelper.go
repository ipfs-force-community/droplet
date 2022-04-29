package network

import (
	"context"
	"os"
	"path/filepath"

	datatransfer "github.com/filecoin-project/go-data-transfer"
	dtimpl "github.com/filecoin-project/go-data-transfer/impl"
	dtnet "github.com/filecoin-project/go-data-transfer/network"
	dtgstransport "github.com/filecoin-project/go-data-transfer/transport/graphsync"
	"github.com/filecoin-project/venus-market/v2/blockstore"
	ds "github.com/ipfs/go-datastore"
	graphsyncimpl "github.com/ipfs/go-graphsync/impl"
	gsnet "github.com/ipfs/go-graphsync/network"
	"github.com/ipfs/go-graphsync/storeutil"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-peerstore/pstoremem"
	mocknet "github.com/libp2p/go-libp2p/p2p/net/mock"
)

func MockHost(ctx context.Context) (host.Host, error) {
	mockNet := mocknet.New()
	ps, err := pstoremem.NewPeerstore()
	if err != nil {
		return nil, err
	}
	pid, err := peer.IDFromString("12D3KooWN1zfzGrxXxTa6ezM3VWxb7Tvqo1R1KXEcxQYV8LC3Em8")
	if err != nil {
		return nil, err
	}
	return mockNet.AddPeerWithPeerstore(pid, ps)
}

func MockDataTransfer(ctx context.Context, h host.Host) (datatransfer.Manager, error) {
	net := dtnet.NewFromLibp2pHost(h)
	graphsyncNetwork := gsnet.NewFromLibp2pHost(h)
	lsys := storeutil.LinkSystemForBlockstore(blockstore.NewMemory())
	gs := graphsyncimpl.New(ctx, graphsyncNetwork, lsys,
		graphsyncimpl.RejectAllRequestsByDefault(),
		graphsyncimpl.MaxInProgressIncomingRequests(10),
		//graphsyncimpl.MaxInProgressIncomingRequestsPerPeer(simultaneousTransfersForStoragePerClient),
		graphsyncimpl.MaxInProgressOutgoingRequests(10),
		graphsyncimpl.MaxLinksPerIncomingRequests(MaxTraversalLinks),
		graphsyncimpl.MaxLinksPerOutgoingRequests(MaxTraversalLinks),
	)

	transport := dtgstransport.NewTransport(h.ID(), gs)
	err := os.MkdirAll(filepath.Join("./", "data-transfer"), 0755) // nolint: gosec
	if err != nil && !os.IsExist(err) {
		return nil, err
	}

	dt, err := dtimpl.NewDataTransfer(ds.NewMapDatastore(), net, transport)
	if err != nil {
		return nil, err
	}
	return dt, nil
}
