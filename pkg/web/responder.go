package web

import (
	"net/http"

	"github.com/go-chi/render"
	"github.com/pkg/errors"

	"backend/pkg/log"
	"backend/pkg/response"
)

type webError struct {
	Code    int         `json:"code,omitempty"`
	Message interface{} `json:"message,omitempty"`
}

type webErrorResponse struct {
	Error *webError `json:"error,omitempty"`
}

type resultResponse struct {
	Result interface{} `json:"result"`
}

// RenderError renders error in JSON format.
func RenderError(w http.ResponseWriter, r *http.Request, err error) {
	log.AddFields(r.Context(), "error", err.Error()) // log the rendered error

	// prepare the error to render
	webErr := &webError{Message: err.Error()}

	cause := errors.Cause(err)
	respErr, ok := cause.(*response.Error)
	if ok {
		webErr.Code = respErr.Code
		webErr.Message = respErr.Message
		if respErr.Internal != nil { // log the internal error
			log.AddFields(r.Context(), "internal", respErr.Internal.Error())
		}
	}

	render.Status(r, http.StatusBadRequest) // default status for errors
	render.JSON(w, r, &webErrorResponse{Error: webErr})
}

func RenderResult(w http.ResponseWriter, r *http.Request, result interface{}) {
	render.JSON(w, r, &resultResponse{Result: result})
}
