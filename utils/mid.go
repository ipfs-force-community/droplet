package utils

import (
	"github.com/google/uuid"
	"github.com/ipfs/go-cid"
	mh "github.com/multiformats/go-multihash"
)

var MidPrefix = cid.Prefix{
	Version:  1,
	Codec:    cid.Raw,
	MhType:   mh.BLAKE2B_MAX,
	MhLength: mh.DefaultLengths[mh.BLAKE2B_MAX], // default length
}

func NewMId() (cid.Cid, error) {
	uid := uuid.New().String()
	// And then feed it some data
	c, err := MidPrefix.Sum([]byte(uid))
	if err != nil {
		return cid.Undef, err
	}

	return c, nil
}

func NewMIdFromBytes(seed []byte) (cid.Cid, error) {

	// And then feed it some data
	c, err := MidPrefix.Sum(seed)
	if err != nil {
		return cid.Undef, err
	}
	return c, nil
}
