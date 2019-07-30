package auth

import (
	"context"
	"net/http"

	"backend/app/models"
)

type Service interface {
	GetJWTVerifier() func(http.Handler) http.Handler
	GetJWTAuthenticator() func(http.Handler) http.Handler
	IssueAccessToken(ctx context.Context, wallet *models.Wallet) (string, error)
}
