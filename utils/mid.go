package utils

import (
	"github.com/google/uuid"
	"github.com/ipfs/go-cid"
	mh "github.com/multiformats/go-multihash"
)

func NewMId() (cid.Cid, error) {
	uid := uuid.New().String()
	pref := cid.Prefix{
		Version:  1,
		Codec:    cid.Raw,
		MhType:   mh.BLAKE2B_MAX,
		MhLength: -1, // default length
	}

	// And then feed it some data
	c, err := pref.Sum([]byte(uid))
	if err != nil {
		return cid.Undef, err
	}
	return c, nil
}

func NewMIdFromBytes(seed []byte) (cid.Cid, error) {
	pref := cid.Prefix{
		Version:  1,
		Codec:    cid.Raw,
		MhType:   mh.BLAKE2B_MAX,
		MhLength: -1, // default length
	}

	// And then feed it some data
	c, err := pref.Sum(seed)
	if err != nil {
		return cid.Undef, err
	}
	return c, nil
}
