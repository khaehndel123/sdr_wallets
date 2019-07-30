package models

import (
	"github.com/pkg/errors"

	"backend/pkg/crypto"
)

type NewWallet struct {
	Address     string `json:"address,omitempty"`
	GeneratedAt int64  `json:"generated_at,omitempty"`
	Signature   string `json:"-"` // provided in a header
}

func (w *NewWallet) Validate(apiSecret string) error {
	if w.Address == "" {
		return errors.New("empty wallet address provided")
	}

	if w.GeneratedAt == 0 {
		return errors.New("empty wallet creation date provided")
	}

	if w.Signature == "" {
		return errors.New("empty signature provided")
	}

	if crypto.GetSHA256(w.Address, apiSecret) != w.Signature {
		return errors.New("invalid signature provided")
	}

	return nil
}

type Wallet struct {
	Base
	NewWallet
}

type AuthorizedWallet struct {
	Wallet      *Wallet `json:"wallet"`
	AccessToken string  `json:"access_token,omitempty"`
}

type GetWallet struct {
	Address   string `json:"address,omitempty"`
	Signature string `json:"-"` // provided in a header
}

func (w *GetWallet) Validate(apiSecret string) error {
	if w.Address == "" {
		return errors.New("empty wallet address provided")
	}

	if w.Signature == "" {
		return errors.New("empty signature provided")
	}

	if crypto.GetSHA256(w.Address, apiSecret) != w.Signature {
		return errors.New("invalid signature provided")
	}

	return nil
}

type Balance struct {
	SDR float64 `json:"sdr"`
}
