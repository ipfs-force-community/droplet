package migrate

import "github.com/ipfs/go-datastore"

type DsKeyAble interface {
	KeyWithNamespace() datastore.Key
}
