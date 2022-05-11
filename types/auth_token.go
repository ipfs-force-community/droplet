package types

import (
	"github.com/ipfs/go-cid"
	"golang.org/x/xerrors"
)

// ErrTokenNotFound is returned when an auth token is not found in the database
var ErrTokenNotFound = xerrors.New("auth token not found")

// AuthValue is the data associated with an auth token in the auth token DB
type AuthValue struct {
	ID          string
	ProposalCid cid.Cid
	PayloadCid  cid.Cid
	Size        uint64
}
