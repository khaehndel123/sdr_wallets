package models

import (
	"context"
	"time"

	"github.com/dgrijalva/jwt-go"
	"github.com/go-chi/jwtauth"
	"github.com/pkg/errors"
)

const (
	accessTokenExpiresIn = time.Hour * 24

	claimWallet = "wallet"
	claimExp    = "exp"
)

type TokenEncoder interface {
	Encode(claims jwtauth.Claims) (t *jwt.Token, tokenString string, err error)
}

type AccessToken struct {
	Wallet    string
	ExpiresAt time.Time
}

func NewAccessToken(wallet string) *AccessToken {
	return &AccessToken{
		Wallet:    wallet,
		ExpiresAt: time.Now().Add(accessTokenExpiresIn),
	}
}

func AccessTokenFromContext(ctx context.Context) (*AccessToken, error) {
	_, claims, err := jwtauth.FromContext(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to retrieve an access token from a context")
	}

	wallet, ok := claims[claimWallet].(string)
	if !ok || wallet == "" {
		return nil, errors.New("empty wallet claim")
	}

	exp, ok := claims[claimExp].(float64)
	if !ok || exp == 0 {
		return nil, errors.New("empty exp claim")
	}

	return &AccessToken{
		Wallet:    wallet,
		ExpiresAt: time.Unix(int64(exp), 0),
	}, nil
}

func (t *AccessToken) Encode(encoder TokenEncoder) (string, error) {
	_, tokenString, err := encoder.Encode(jwtauth.Claims{
		claimWallet: t.Wallet,
		claimExp:    t.ExpiresAt.Unix(),
	})
	return tokenString, errors.Wrap(err, "failed to encode a jwt")
}
