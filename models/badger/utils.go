package badger

import (
	"bytes"
	"context"
	"fmt"
	"reflect"
	"time"

	cborrpc "github.com/filecoin-project/go-cbor-util"
	types "github.com/filecoin-project/venus/venus-shared/types/market"
	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/query"
	cbg "github.com/whyrusleeping/cbor-gen"
)

func checkCallbackAndGetParamType(i interface{}) (reflect.Type, error) {
	t := reflect.TypeOf(i)
	if t.Kind() != reflect.Func {
		return nil, fmt.Errorf("must be a function")
	}
	if t.NumIn() != 1 {
		return nil, fmt.Errorf("callback must and only have 1 param")
	}
	if t.NumOut() != 2 {
		return nil, fmt.Errorf("callback must and only have 2 return value")
	}
	in := t.In(0)
	if !in.Implements(reflect.TypeOf((*cbg.CBORUnmarshaler)(nil)).Elem()) {
		return nil, fmt.Errorf("param must be a CBORUnmarshaler")
	}
	if t.Out(0).Kind() != reflect.Bool {
		return nil, fmt.Errorf("1st return value must be an boolean")
	}
	if !t.Out(1).Implements(reflect.TypeOf((*error)(nil)).Elem()) {
		return nil, fmt.Errorf("return value must be an error interface")
	}
	return in.Elem(), nil
}

func travelDeals(ctx context.Context, ds datastore.Batching, callback interface{}) error {
	instanceType, err := checkCallbackAndGetParamType(callback)
	if err != nil {
		return err
	}

	result, err := ds.Query(ctx, query.Query{})
	if err != nil {
		return err
	}

	defer result.Close() //nolint:errcheck

	for res := range result.Next() {
		if res.Error != nil {
			return err
		}
		i := reflect.New(instanceType).Interface()
		unmarshaler := i.(cbg.CBORUnmarshaler)
		if err = cborrpc.ReadCborRPC(bytes.NewReader(res.Value), unmarshaler); err != nil {
			return err
		}
		rets := reflect.ValueOf(callback).Call([]reflect.Value{
			reflect.ValueOf(unmarshaler)})

		if rets[0].Interface().(bool) {
			return nil
		}

		if !rets[1].IsNil() {
			return rets[0].Interface().(error)
		}
	}
	return nil
}

func makeRefreshedTimeStamp(ts *types.TimeStamp) types.TimeStamp {
	var newTs types.TimeStamp
	if ts != nil {
		newTs = *ts
	}
	newTs.UpdatedAt = uint64(time.Now().Unix())
	if newTs.CreatedAt == 0 {
		newTs.CreatedAt = newTs.UpdatedAt
	}
	return newTs
}
