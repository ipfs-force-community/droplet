package rpc

import (
	"context"
	"crypto/rand"
	"io"
	"io/ioutil"

	auth2 "github.com/filecoin-project/go-jsonrpc/auth"
	"github.com/filecoin-project/venus-auth/auth"
	"github.com/filecoin-project/venus-auth/core"
	"github.com/filecoin-project/venus-auth/jwtclient"
	jwt3 "github.com/gbrlsnchs/jwt/v3"
)

type JwtClient struct {
	alg *jwt3.HMACSHA
}

func NewJwtClient(secret []byte) *JwtClient {
	return &JwtClient{
		alg: jwt3.NewHS256(secret),
	}
}

func (c *JwtClient) Verify(ctx context.Context, token string) ([]auth2.Permission, error) {
	var payload auth.JWTPayload
	_, err := jwt3.Verify([]byte(token), c.alg, &payload)
	if err != nil {
		return nil, err
	}
	jwtPerms := core.AdaptOldStrategy(payload.Perm)
	perms := make([]auth2.Permission, len(jwtPerms))
	copy(perms, jwtPerms)

	return perms, nil
}

func (c *JwtClient) NewAuth(payload auth.JWTPayload) ([]byte, error) {
	return jwt3.Sign(payload, c.alg)
}

func RandSecret() ([]byte, error) {
	return ioutil.ReadAll(io.LimitReader(rand.Reader, 32))
}

var _ jwtclient.IJwtAuthClient = (*JwtClient)(nil)
