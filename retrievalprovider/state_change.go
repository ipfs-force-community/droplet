package retrievalprovider

import (
	rm "github.com/filecoin-project/go-fil-markets/retrievalmarket"
	"github.com/filecoin-project/go-statemachine/fsm"
	"github.com/modern-go/reflect2"

	"sync"
	"unsafe"

	"github.com/filecoin-project/go-fil-markets/retrievalmarket/impl/providerstates"
)

// some code rely on event and current status, todo remote state chang
type StateChange struct {
	states map[rm.ProviderEvent]map[rm.DealStatus]rm.DealStatus
	lk     sync.RWMutex
}

func NewStateChange() *StateChange {
	states := map[rm.ProviderEvent]map[rm.DealStatus]rm.DealStatus{}
	for _, event := range providerstates.ProviderEvents {
		eve := (*struct {
			name             interface{}
			action           fsm.ActionFunc
			transitionsSoFar map[fsm.StateKey]fsm.StateKey
		})((*struct {
			rtype unsafe.Pointer
			data  unsafe.Pointer
		})(unsafe.Pointer(&event)).data)
		fromTo := map[rm.DealStatus]rm.DealStatus{}
		states[eve.name.(rm.ProviderEvent)] = fromTo
		for key, val := range eve.transitionsSoFar {
			if !reflect2.IsNil(key) && !reflect2.IsNil(val) {
				if _, ok := val.(rm.DealStatus); ok {
					fromTo[key.(rm.DealStatus)] = val.(rm.DealStatus)
				}
			}
		}
	}
	return &StateChange{
		states: states,
		lk:     sync.RWMutex{},
	}
}

func (sc *StateChange) Get(event rm.ProviderEvent, curStatus rm.DealStatus) (rm.DealStatus, bool) {
	sc.lk.RLock()
	defer sc.lk.Unlock()
	changes, ok := sc.states[event]
	if !ok {
		return rm.DealStatusNew, false
	}
	dstStatus, ok := changes[curStatus]
	if !ok {
		return rm.DealStatusNew, false
	}
	return dstStatus, true
}
