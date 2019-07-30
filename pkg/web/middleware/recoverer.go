package middleware

import (
	"net/http"

	"github.com/go-chi/render"

	"backend/pkg/log"
)

var (
	errInternal = http.StatusText(http.StatusInternalServerError)
)

type internalError struct {
	Code    int         `json:"code,omitempty"`
	Message interface{} `json:"message,omitempty"`
}

type internalErrorResponse struct {
	Error *internalError `json:"error,omitempty"`
}

func Recoverer(next http.Handler) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rvr := recover(); rvr != nil {
				log.Error(rvr) // print stack trace if allowed
				render.Status(r, http.StatusInternalServerError)
				render.JSON(w, r, &internalErrorResponse{
					Error: &internalError{Message: errInternal},
				})
			}
		}()

		next.ServeHTTP(w, r)
	}
	return http.HandlerFunc(fn)
}
