package auth

import (
	"context"
	"net/http"

	"github.com/go-chi/jwtauth"

	"backend/app/models"
	"backend/pkg/log"
	"backend/pkg/response"
	"backend/pkg/web"
)

type Manager struct {
	JWTAuth *jwtauth.JWTAuth
}

func (m *Manager) GetJWTVerifier() func(http.Handler) http.Handler {
	return jwtauth.Verifier(m.JWTAuth)
}

func (m *Manager) GetJWTAuthenticator() func(http.Handler) http.Handler {
	return Authenticator
}

func (m *Manager) IssueAccessToken(ctx context.Context, wallet *models.Wallet) (string, error) {
	log.AddFields(ctx, "issue token for", wallet.Address)

	accessToken := models.NewAccessToken(wallet.Address)
	return accessToken.Encode(m.JWTAuth)
}

func Authenticator(next http.Handler) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		token, _, err := jwtauth.FromContext(r.Context())

		if err != nil {
			web.RenderError(w, r, response.NewError(response.CodeUnauthorized, err.Error()))
			return
		}

		if token == nil || !token.Valid {
			web.RenderError(
				w, r, response.NewError(response.CodeUnauthorized, http.StatusText(http.StatusUnauthorized)),
			)
			return
		}

		// token is authenticated, pass it through
		next.ServeHTTP(w, r)
	}
	return http.HandlerFunc(fn)
}
