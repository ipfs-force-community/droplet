package idxprov

import (
	"context"
	"fmt"

	v1api "github.com/filecoin-project/venus/venus-shared/api/chain/v1"
	"github.com/libp2p/go-libp2p-core/host"
)

const protectTag = "index-provider-gossipsub"

type MeshCreator interface {
	Connect(ctx context.Context) error
}

type Libp2pMeshCreator struct {
	fullnodeApi v1api.FullNode
	marketsHost host.Host
}

func (mc Libp2pMeshCreator) Connect(ctx context.Context) error {

	// Add the markets host ID to list of daemon's protected peers first, before any attempt to
	// connect to full node over libp2p.
	marketsPeerID := mc.marketsHost.ID()

	// todo: venus doesn't have a interface to `protect peer id`
	//if err := mc.fullnodeApi.NetProtectAdd(ctx, []peer.ID{marketsPeerID}); err != nil {
	//	return fmt.Errorf("failed to call NetProtectAdd on the full node, err: %w", err)
	//}

	faddrs, err := mc.fullnodeApi.NetAddrsListen(ctx)
	if err != nil {
		return fmt.Errorf("failed to fetch full node listen addrs, err: %w", err)
	}

	// Connect to the full node, ask it to protect the connection and protect the connection on
	// markets end too.
	if err := mc.marketsHost.Connect(ctx, faddrs); err != nil {
		return fmt.Errorf("failed to connect index provider host with the full node: %w", err)
	}
	mc.marketsHost.ConnManager().Protect(faddrs.ID, protectTag)

	log.Debugw("successfully connected to full node and asked it protect indexer provider peer conn", "fullNodeInfo", faddrs.String(),
		"peerId", marketsPeerID)

	return nil
}

func NewMeshCreator(fullnodeApi v1api.FullNode, marketsHost host.Host) MeshCreator {
	return Libp2pMeshCreator{fullnodeApi, marketsHost}
}
