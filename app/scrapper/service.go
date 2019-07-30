package scrapper

import (
	"context"

	"backend/app/models"
)

type Service interface {
	CreateTransaction(ctx context.Context, transaction *models.NewTransaction) (*models.Transaction, error)
	NotifyClients(ctx context.Context, transaction *models.NewTransaction)
	IsWalletExist(ctx context.Context, from, to string) bool
	TxHasQueuedTransfers(ctx context.Context, txHash string) (string, error)
	FailEthTransfer(ctx context.Context, id, msg string) error
	CompleteTransfer(ctx context.Context, transferHash string) error
	CompleteTransfers(ctx context.Context, ethTransferID string) error
	RegisterWallet(ctx context.Context, address string)
	GetPendingTopUps(ctx context.Context) ([]*models.DBTransfer, error)
	GetPendingTransfers(ctx context.Context) ([]*models.DBTransfer, error)
}
