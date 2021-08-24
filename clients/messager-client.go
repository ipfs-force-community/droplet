package clients

import (
	"context"
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-jsonrpc"
	"github.com/filecoin-project/venus-market/config"
	"github.com/filecoin-project/venus-market/metrics"
	"github.com/filecoin-project/venus-messager/types"
	vTypes "github.com/filecoin-project/venus/pkg/types"
	"go.uber.org/fx"
	"golang.org/x/xerrors"
)

var ErrFailMsg = xerrors.New("Message Fail")

type IMessager interface {
	WalletHas(ctx context.Context, addr address.Address) (bool, error)
	WaitMessage(ctx context.Context, id string, confidence uint64) (*types.Message, error)
	PushMessage(ctx context.Context, msg *vTypes.Message, meta *types.MsgMeta) (string, error)
	PushMessageWithId(ctx context.Context, id string, msg *vTypes.Message, meta *types.MsgMeta) (string, error)
	GetMessageByUid(ctx context.Context, id string) (*types.Message, error)
}

var _ IMessager = (*Messager)(nil)

type Messager struct {
}

func NewMessager() *Messager {
	return nil
}

func (message *Messager) WaitMessage(ctx context.Context, id string, confidence uint64) (*types.Message, error) {
	panic("to impl")
}

func (m *Messager) WalletHas(ctx context.Context, addr address.Address) (bool, error) {
	panic("to impl")
}

func (m *Messager) PushMessage(ctx context.Context, msg *vTypes.Message, meta *types.MsgMeta) (string, error) {
	panic("to impl")
}

func (m *Messager) PushMessageWithId(ctx context.Context, id string, msg *vTypes.Message, meta *types.MsgMeta) (string, error) {
	panic("to impl")
}

func (m *Messager) GetMessageByUid(ctx context.Context, id string) (*types.Message, error) {
	panic("to impl")
}

func NewMessageRPC(messagerCfg *config.Messager) (IMessager, jsonrpc.ClientCloser, error) {
	panic("to impl")
}

func MessagerClient(mctx metrics.MetricsCtx, lc fx.Lifecycle, nodeCfg *config.Messager) (IMessager, error) {
	client, closer, err := NewMessageRPC(nodeCfg)

	lc.Append(fx.Hook{
		OnStop: func(_ context.Context) error {
			closer()
			return nil
		},
	})
	return client, err
}
