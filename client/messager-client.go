package client

import (
	"context"
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-jsonrpc"
	"github.com/filecoin-project/venus-market/config"
	"github.com/filecoin-project/venus-messager/api/client"
	"github.com/filecoin-project/venus-messager/types"
	types2 "github.com/filecoin-project/venus/pkg/types"
	"golang.org/x/xerrors"
	"net/http"
)

var ErrFailMsg = xerrors.New("Message Fail")

type IMessager interface {
	HasWalletAddress(ctx context.Context, addr address.Address) (bool, error)
	WaitMessage(ctx context.Context, id string, confidence uint64) (*types.Message, error)
	PushMessage(ctx context.Context, msg *types2.UnsignedMessage, meta *types.MsgMeta) (string, error)
	PushMessageWithId(ctx context.Context, id string, msg *types2.UnsignedMessage, meta *types.MsgMeta) (string, error)
	GetMessageByUid(ctx context.Context, id string) (*types.Message, error)
}

var _ IMessager = (*Messager)(nil)

type Messager struct {
	in         client.IMessager
	walletName string
}

func NewMessager(in client.IMessager, walletName string) *Messager {
	return &Messager{in: in, walletName: walletName}
}

func (m *Messager) WaitMessage(ctx context.Context, id string, confidence uint64) (*types.Message, error) {
	msg, err := m.in.WaitMessage(ctx, id, confidence)
	if err != nil {
		return nil, err
	}
	if msg.State == types.FailedMsg {
		return nil, ErrFailMsg
	}
	return msg, nil
}

func (m *Messager) HasWalletAddress(ctx context.Context, addr address.Address) (bool, error) {
	return m.in.HasWalletAddress(ctx, m.walletName, addr)
}

func (m *Messager) PushMessage(ctx context.Context, msg *types2.UnsignedMessage, meta *types.MsgMeta) (string, error) {
	return m.in.PushMessage(ctx, msg, meta, m.walletName)
}

func (m *Messager) PushMessageWithId(ctx context.Context, id string, msg *types2.UnsignedMessage, meta *types.MsgMeta) (string, error) {
	return m.in.PushMessageWithId(ctx, id, msg, meta, m.walletName)
}

func (m *Messager) GetMessageByUid(ctx context.Context, id string) (*types.Message, error) {
	return m.in.GetMessageByUid(ctx, id)
}

func NewMessageRPC(ctx context.Context, messagerCfg *config.MessageServiceConfig) (IMessager, jsonrpc.ClientCloser, error) {
	headers := http.Header{}
	if len(messagerCfg.Token) != 0 {
		headers.Add("Authorization", "Bearer "+messagerCfg.Token)
	}

	client, closer, err := client.NewMessageRPC(ctx, messagerCfg.Url, headers)
	if err != nil {
		return nil, nil, err
	}

	return NewMessager(client, messagerCfg.Wallet), closer, nil
}
