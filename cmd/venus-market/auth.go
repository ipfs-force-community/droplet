package main

import (
	"context"
	"crypto/rand"
	"io"
	"io/ioutil"

	"github.com/filecoin-project/go-jsonrpc/auth"
	auth2 "github.com/filecoin-project/venus-auth/auth"
	"github.com/filecoin-project/venus-auth/cmd/jwtclient"
	"github.com/filecoin-project/venus-auth/core"
	jwt3 "github.com/gbrlsnchs/jwt/v3"
	xerrors "github.com/pkg/errors"
)

// todo: this is a temporary solution
type localJwtClient struct{ seckey []byte }

func (l *localJwtClient) Verify(ctx context.Context, token string) ([]auth.Permission, error) {
	var payload auth2.JWTPayload
	if _, err := jwt3.Verify([]byte(token), jwt3.NewHS256(l.seckey), &payload); err != nil {
		return nil, xerrors.Errorf("JWT Verification failed: %v", err)
	}
	jwtPerms := core.AdaptOldStrategy(payload.Perm)
	perms := make([]auth.Permission, len(jwtPerms))
	copy(perms, jwtPerms)
	return perms, nil
}

var _ jwtclient.IJwtAuthClient = (*localJwtClient)(nil)

func MakeToken() ([]byte, error) {
	const tokenFile = "./token"
	var err error
	var seckey []byte
	if seckey, err = ioutil.ReadAll(io.LimitReader(rand.Reader, 32)); err != nil {
		return nil, err
	}
	var cliToken []byte
	if cliToken, err = jwt3.Sign(
		auth2.JWTPayload{
			Perm: core.PermAdmin,
			Name: "GateWayLocalToken",
		}, jwt3.NewHS256(seckey)); err != nil {
		return nil, err
	}
	return seckey, ioutil.WriteFile(tokenFile, cliToken, 0644)
}
