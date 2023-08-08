package models

import (
	"context"

	"github.com/filecoin-project/go-address"
	"github.com/ipfs-force-community/sophon-auth/auth"
	"github.com/ipfs-force-community/sophon-auth/core"
	"github.com/ipfs-force-community/sophon-auth/jwtclient"
)

type IAuthClientStub struct {
}

var _ jwtclient.IAuthClient = (*IAuthClientStub)(nil)

func (*IAuthClientStub) MinerExistInUser(ctx context.Context, user string, miner address.Address) (bool, error) {
	return true, nil
}

func (*IAuthClientStub) SignerExistInUser(ctx context.Context, user string, signer address.Address) (bool, error) {
	return true, nil
}

func (ia *IAuthClientStub) Verify(ctx context.Context, token string) (*auth.VerifyResponse, error) {
	panic("not implemented") // TODO: Implement
}

func (ia *IAuthClientStub) VerifyUsers(ctx context.Context, names []string) error {
	panic("not implemented") // TODO: Implement
}

func (ia *IAuthClientStub) HasUser(ctx context.Context, name string) (bool, error) {
	panic("not implemented") // TODO: Implement
}

func (ia *IAuthClientStub) GetUser(ctx context.Context, name string) (*auth.OutputUser, error) {
	panic("not implemented") // TODO: Implement
}

func (ia *IAuthClientStub) GetUserByMiner(ctx context.Context, miner address.Address) (*auth.OutputUser, error) {
	panic("not implemented") // TODO: Implement
}

func (ia *IAuthClientStub) GetUserBySigner(ctx context.Context, signer address.Address) (auth.ListUsersResponse, error) {
	panic("not implemented") // TODO: Implement
}

func (ia *IAuthClientStub) ListUsers(ctx context.Context, skip int64, limit int64, state core.UserState) (auth.ListUsersResponse, error) {
	panic("not implemented") // TODO: Implement
}

func (ia *IAuthClientStub) ListUsersWithMiners(ctx context.Context, skip int64, limit int64, state core.UserState) (auth.ListUsersResponse, error) {
	panic("not implemented") // TODO: Implement
}

func (ia *IAuthClientStub) GetUserRateLimit(ctx context.Context, name string, id string) (auth.GetUserRateLimitResponse, error) {
	panic("not implemented") // TODO: Implement
}

func (ia *IAuthClientStub) HasMiner(ctx context.Context, miner address.Address) (bool, error) {
	panic("not implemented") // TODO: Implement
}

func (ia *IAuthClientStub) ListMiners(ctx context.Context, user string) (auth.ListMinerResp, error) {
	panic("not implemented") // TODO: Implement
}

func (ia *IAuthClientStub) UpsertMiner(ctx context.Context, user string, miner string, openMining bool) (bool, error) {
	panic("not implemented") // TODO: Implement
}

func (ia *IAuthClientStub) HasSigner(ctx context.Context, signer address.Address) (bool, error) {
	panic("not implemented") // TODO: Implement
}

func (ia *IAuthClientStub) ListSigners(ctx context.Context, user string) (auth.ListSignerResp, error) {
	panic("not implemented") // TODO: Implement
}

func (ia *IAuthClientStub) RegisterSigners(ctx context.Context, user string, addrs []address.Address) error {
	panic("not implemented") // TODO: Implement
}

func (ia *IAuthClientStub) UnregisterSigners(ctx context.Context, user string, addrs []address.Address) error {
	panic("not implemented") // TODO: Implement
}
