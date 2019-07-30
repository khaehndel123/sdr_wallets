package database

import (
	"context"
)

type Database interface {
	CreateWallet(ctx context.Context, wallet *NewWallet) (*Wallet, error)
	GetWallet(ctx context.Context, address string) (*Wallet, error)
	CreateTransaction(ctx context.Context, transaction *NewTransaction) (*Transaction, error)
	ExistingWallet(ctx context.Context, from, to string) string
	TxHasQueuedTransfers(ctx context.Context, txHash string) (string, error)
	CreateTransfers(ctx context.Context, transfers []*NewTransfer) ([]*Transfer, error)
	GetAndConfirmTransferByID(ctx context.Context, id, from string) (*Transfer, error)
	GetTransferByIDOrHash(ctx context.Context, id, hash string) (*Transfer, error)
	UpdateTransfers(ctx context.Context, transfers []*Transfer) ([]*Transfer, error)
	CreateEthTransfer(ctx context.Context, transfer *NewTransfer, txHash string) (*Transfer, error)
	StartEthTransferProcessing(ctx context.Context, id string) error
	CompleteTransfer(ctx context.Context, id, hash string) error
	FailEthTransfer(ctx context.Context, id, msg string) error
	WriteTransferError(ctx context.Context, id, msg string) error
	GetQueuedTransfersFor(ctx context.Context, ethTransferID string) ([]*Transfer, error)
	IsNoncePending(ctx context.Context, nonce uint64, address string) bool
	TransactionHistory(ctx context.Context, filter *TransactionHistoryFilter) ([]*TransactionHistoryItem, uint64, error)
	GetTransactionOrTransfer(ctx context.Context, filter *TransactionFilter) (*TransactionHistoryItem, error)
	GetPendingTopUps(ctx context.Context) ([]*Transfer, error)
	GetPendingTransfers(ctx context.Context) ([]*Transfer, error)
	ClearUnsignedTransfers(ctx context.Context, address string) error
}
