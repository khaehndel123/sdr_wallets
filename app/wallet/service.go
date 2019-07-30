package wallet

import (
	"context"

	"backend/app/models"
)

type Service interface {
	CreateWallet(ctx context.Context, wallet *models.NewWallet) (*models.AuthorizedWallet, error)
	GetWallet(ctx context.Context, filter *models.GetWallet) (*models.AuthorizedWallet, error)
	GetBalance(ctx context.Context, address string) (*models.Balance, error)
}
