package badger

import (
	"bytes"

	"github.com/filecoin-project/go-address"
	cborrpc "github.com/filecoin-project/go-cbor-util"
	"github.com/ipfs/go-datastore"
	dsq "github.com/ipfs/go-datastore/query"

	"github.com/filecoin-project/venus-market/types"
)

const dsKeyAddr = "Addr"

type fundRepo struct {
	ds datastore.Batching
}

func NewFundRepo(ds FundMgrDS) *fundRepo {
	return &fundRepo{
		ds: ds,
	}
}

// SaveFundedAddressState save the state to the datastore
func (fr *fundRepo) SaveFundedAddressState(state *types.FundedAddressState) error {
	k := dskeyForAddr(state.Addr)

	b, err := cborrpc.Dump(state)
	if err != nil {
		return err
	}

	return fr.ds.Put(k, b)
}

// GetFundedAddressState get the state for the given address
func (fr *fundRepo) GetFundedAddressState(addr address.Address) (*types.FundedAddressState, error) { //nolint
	k := dskeyForAddr(addr)

	data, err := fr.ds.Get(k)
	if err != nil {
		return nil, err
	}

	var state types.FundedAddressState
	err = cborrpc.ReadCborRPC(bytes.NewReader(data), &state)
	if err != nil {
		return nil, err
	}
	return &state, nil
}

// ListFundedAddressState get all states in the datastore
func (fr *fundRepo) ListFundedAddressState() ([]*types.FundedAddressState, error) {
	res, err := fr.ds.Query(dsq.Query{Prefix: dsKeyAddr})
	if err != nil {
		return nil, err
	}
	defer res.Close() //nolint:errcheck

	fas := make([]*types.FundedAddressState, 0)
	for {
		res, ok := res.NextSync()
		if !ok {
			break
		}

		if res.Error != nil {
			return nil, err
		}

		var stored types.FundedAddressState
		if err := stored.UnmarshalCBOR(bytes.NewReader(res.Value)); err != nil {
			return nil, err
		}
		fas = append(fas, &stored)
	}

	return fas, nil
}

// The datastore key used to identify the address state
func dskeyForAddr(addr address.Address) datastore.Key {
	return datastore.KeyWithNamespaces([]string{dsKeyAddr, addr.String()})
}
