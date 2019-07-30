package server

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi"
	"github.com/go-chi/render"

	"backend/app/auth"
	"backend/app/models"
	"backend/app/notifier"
	"backend/app/transaction"
	"backend/app/wallet"
	"backend/pkg/web"
)

const (
	apiPrefix       = "/api/v1"
	signatureHeader = "x-signature"
)

// Rest is a gateway for incoming HTTP requests
type Rest struct {
	Router      chi.Router
	Wallet      wallet.Service
	Notifier    notifier.Service
	Auth        auth.Service
	Transaction transaction.Service
}

func (s *Rest) Route() {
	s.Router.Route(apiPrefix, func(r chi.Router) {
		// semi-public routes (signature required)
		r.Route("/wallet", func(r chi.Router) {
			r.Post("/", s.createWallet)
			r.Get("/{address}", s.getWallet)
		})

		// private routes
		r.Group(func(r chi.Router) {
			r.Use(s.Auth.GetJWTVerifier(), s.Auth.GetJWTAuthenticator())

			r.Get("/subscribe", s.subscribe)

			r.Post("/estimatetax", s.estimateTax)
			r.Post("/transfer", s.createTransfer)
			r.Post("/transfer/confirm", s.confirmTransfer)

			r.Get("/transactions", s.transactionHistory)
			r.Get("/transactions/{hash}", s.getTransaction)

			r.Get("/balance", s.getBalance)
		})
	})
}

func (s *Rest) createWallet(w http.ResponseWriter, r *http.Request) {
	in := new(models.NewWallet)
	if err := render.DecodeJSON(r.Body, in); err != nil {
		web.RenderError(w, r, err)
		return
	}
	in.Signature = r.Header.Get(signatureHeader)

	out, err := s.Wallet.CreateWallet(r.Context(), in)
	if err != nil {
		web.RenderError(w, r, err)
		return
	}

	web.RenderResult(w, r, out)
}

func (s *Rest) getWallet(w http.ResponseWriter, r *http.Request) {
	address := chi.URLParam(r, "address")
	in := &models.GetWallet{Address: address}
	in.Signature = r.Header.Get(signatureHeader)

	out, err := s.Wallet.GetWallet(r.Context(), in)
	if err != nil {
		web.RenderError(w, r, err)
		return
	}

	web.RenderResult(w, r, out)
}

func (s *Rest) subscribe(w http.ResponseWriter, r *http.Request) {
	accessToken, err := models.AccessTokenFromContext(r.Context())
	if err != nil {
		web.RenderError(w, r, err)
		return
	}

	if err := s.Notifier.Subscribe(r.Context(), &models.NewSubscription{
		ClientID:       accessToken.Wallet,
		ResponseWriter: w,
		Request:        r,
	}); err != nil {
		web.RenderError(w, r, err)
		return
	}
}

func (s *Rest) estimateTax(w http.ResponseWriter, r *http.Request) {
	accessToken, err := models.AccessTokenFromContext(r.Context())
	if err != nil {
		web.RenderError(w, r, err)
		return
	}

	in := new(models.NewTransfer)
	if err := render.DecodeJSON(r.Body, in); err != nil {
		web.RenderError(w, r, err)
		return
	}
	in.FromAddress = accessToken.Wallet

	out, err := s.Transaction.EstimateTax(r.Context(), in)
	if err != nil {
		web.RenderError(w, r, err)
		return
	}

	web.RenderResult(w, r, out)
}

func (s *Rest) createTransfer(w http.ResponseWriter, r *http.Request) {
	accessToken, err := models.AccessTokenFromContext(r.Context())
	if err != nil {
		web.RenderError(w, r, err)
		return
	}

	in := new(models.NewTransfer)
	if err := render.DecodeJSON(r.Body, in); err != nil {
		web.RenderError(w, r, err)
		return
	}
	in.FromAddress = accessToken.Wallet

	out, err := s.Transaction.CreateTransfer(r.Context(), in)
	if err != nil {
		web.RenderError(w, r, err)
		return
	}

	web.RenderResult(w, r, out)
}

func (s *Rest) confirmTransfer(w http.ResponseWriter, r *http.Request) {
	accessToken, err := models.AccessTokenFromContext(r.Context())
	if err != nil {
		web.RenderError(w, r, err)
		return
	}

	in := new(models.SignedTransfer)
	if err := render.DecodeJSON(r.Body, in); err != nil {
		web.RenderError(w, r, err)
		return
	}
	in.FromAddress = accessToken.Wallet

	out, err := s.Transaction.ConfirmTransfer(r.Context(), in)
	if err != nil {
		web.RenderError(w, r, err)
		return
	}

	web.RenderResult(w, r, out)
}

func (s *Rest) transactionHistory(w http.ResponseWriter, r *http.Request) {
	accessToken, err := models.AccessTokenFromContext(r.Context())
	if err != nil {
		web.RenderError(w, r, err)
		return
	}

	var after uint64
	qafter, ok := r.URL.Query()["after"]
	if ok && len(qafter) > 0 {
		after, _ = strconv.ParseUint(qafter[0], 10, 64)
	}

	var skip uint64
	qskip, ok := r.URL.Query()["skip"]
	if ok && len(qskip) > 0 {
		skip, _ = strconv.ParseUint(qskip[0], 10, 64)
	}

	var limit *uint64
	qlimit, ok := r.URL.Query()["limit"]
	if ok && len(qlimit) > 0 {
		tmpLimit, _ := strconv.ParseUint(qlimit[0], 10, 64)
		limit = &tmpLimit
	}

	in := &models.TransactionHistoryFilter{
		Address: accessToken.Wallet,
		After:   after,
		Skip:    skip,
		Limit:   limit,
	}

	out, err := s.Transaction.TransactionHistory(r.Context(), in)
	if err != nil {
		web.RenderError(w, r, err)
		return
	}

	web.RenderResult(w, r, out)
}

func (s *Rest) getTransaction(w http.ResponseWriter, r *http.Request) {
	accessToken, err := models.AccessTokenFromContext(r.Context())
	if err != nil {
		web.RenderError(w, r, err)
		return
	}

	in := &models.TransactionFilter{
		Address: accessToken.Wallet,
		Hash:    chi.URLParam(r, "hash"),
	}

	out, err := s.Transaction.GetTransaction(r.Context(), in)
	if err != nil {
		web.RenderError(w, r, err)
		return
	}

	web.RenderResult(w, r, out)
}

func (s *Rest) getBalance(w http.ResponseWriter, r *http.Request) {
	accessToken, err := models.AccessTokenFromContext(r.Context())
	if err != nil {
		web.RenderError(w, r, err)
		return
	}

	out, err := s.Wallet.GetBalance(r.Context(), accessToken.Wallet)
	if err != nil {
		web.RenderError(w, r, err)
		return
	}

	web.RenderResult(w, r, out)
}
