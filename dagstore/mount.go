package dagstore

import (
	"context"
	"fmt"
	"net/url"

	"github.com/ipfs/go-cid"

	"github.com/filecoin-project/dagstore/mount"
)

const marketScheme = "market"

var _ mount.Mount = (*PieceMount)(nil)

// mountTemplate returns a templated PieceMount containing the supplied API.
//
// It is called when registering a mount type with the mount registry
// of the DAG store. It is used to reinstantiate mounts after a restart.
//
// When the registry needs to deserialize a mount it clones the template then
// calls Deserialize on the cloned instance, which will have a reference to the
// piece mount API supplied here.

// in current design, we cannot assign different mount type by different storage tpye
// todo support mount type for different storage type
func mountTemplate(api MarketAPI, useTransient bool) *PieceMount {
	return &PieceMount{API: api, UseTransient: useTransient}
}

// PieceMount is a DAGStore mount implementation that fetches deal data
// from a PieceCID.
type PieceMount struct {
	API          MarketAPI
	PieceCid     cid.Cid
	UseTransient bool //must use public, dagstore reflect field and set value from template
}

func NewPieceMount(pieceCid cid.Cid, useTransient bool, api MarketAPI) (*PieceMount, error) {
	return &PieceMount{
		PieceCid:     pieceCid,
		API:          api,
		UseTransient: useTransient,
	}, nil
}

func (l *PieceMount) Serialize() *url.URL {
	return &url.URL{
		Host: l.PieceCid.String(),
	}
}

func (l *PieceMount) Deserialize(u *url.URL) error {
	pieceCid, err := cid.Decode(u.Host)
	if err != nil {
		return fmt.Errorf("failed to parse PieceCid from host '%s': %w", u.Host, err)
	}
	l.PieceCid = pieceCid
	//l.UseTransient = l.useTransient
	return nil
}

func (l *PieceMount) Fetch(ctx context.Context) (mount.Reader, error) {
	r, err := l.API.FetchFromPieceStorage(ctx, l.PieceCid)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch unsealed piece %s: %w", l.PieceCid, err)
	}
	return r, nil
}

func (l *PieceMount) Info() mount.Info {
	if l.UseTransient {
		return mount.Info{
			Kind:             mount.KindRemote,
			AccessSequential: true,
			AccessSeek:       false,
			AccessRandom:     false,
		}
	}

	return mount.Info{
		Kind:             mount.KindRemote,
		AccessSequential: true,
		AccessSeek:       true,
		AccessRandom:     true,
	}
}

func (l *PieceMount) Close() error {
	return nil
}

func (l *PieceMount) Stat(ctx context.Context) (mount.Stat, error) {
	size, err := l.API.GetUnpaddedCARSize(ctx, l.PieceCid)
	if err != nil {
		return mount.Stat{}, fmt.Errorf("failed to fetch piece size for piece %s: %w", l.PieceCid, err)
	}
	isUnsealed, err := l.API.IsUnsealed(ctx, l.PieceCid)
	if err != nil {
		return mount.Stat{}, fmt.Errorf("failed to verify if we have the unsealed piece %s: %w", l.PieceCid, err)
	}

	// TODO Mark false when storage deal expires.
	return mount.Stat{
		Exists: true,
		Size:   int64(size),
		Ready:  isUnsealed,
	}, nil
}
