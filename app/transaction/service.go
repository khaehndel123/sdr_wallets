package transaction

import (
	"context"

	"backend/app/models"
)

type Service interface {
	EstimateTax(ctx context.Context, tx *models.NewTransfer) (*models.TaxEstimation, error)
	CreateTransfer(ctx context.Context, tx *models.NewTransfer) ([]*models.UnsignedTransfer, error)
	ConfirmTransfer(ctx context.Context, transfers *models.SignedTransfer) (*models.QueuedTransfer, error)
	CompleteTransfers(ctx context.Context, ethTransferID string) error
	TransactionHistory(ctx context.Context, filter *models.TransactionHistoryFilter) (*models.TransactionHistory, error)
	GetTransaction(ctx context.Context, filter *models.TransactionFilter) (*models.TransactionHistoryItem, error)
}
