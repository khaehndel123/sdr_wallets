package scrapper

import (
	"context"
	"time"

	"github.com/patrickmn/go-cache"

	"backend/app/config"
	"backend/app/models"
	"backend/app/notifier"
	"backend/app/storage/database"
	"backend/app/transaction"
	"backend/pkg/eth"
)

const (
	decimals        = 18
	cacheExpiration = 5 * time.Minute
	cleanupInterval = 7 * time.Minute
)

type Manager struct {
	EthConfig   config.Ethereum
	DB          database.Database
	Transaction transaction.Service
	Notifier    notifier.Service

	cache *cache.Cache
}

func NewManager(
	cfg config.Ethereum,
	db database.Database,
	transaction transaction.Service,
	notifier notifier.Service,
) *Manager {
	return &Manager{
		EthConfig:   cfg,
		DB:          db,
		Transaction: transaction,
		Notifier:    notifier,
		cache:       cache.New(cacheExpiration, cleanupInterval),
	}
}

func (m *Manager) CreateTransaction(ctx context.Context, transaction *models.NewTransaction) (*models.Transaction, error) {
	if err := transaction.Validate(); err != nil {
		return nil, err
	}

	dbTransaction, err := m.DB.CreateTransaction(ctx, database.NewTransactionFromPublic(transaction))
	if err != nil {
		return nil, err
	}

	return dbTransaction.ToPublic(), nil
}

func (m *Manager) NotifyClients(ctx context.Context, transaction *models.NewTransaction) {
	// notify clients about the transaction
	amount := eth.ToETH(transaction.Value, decimals)
	famount, _ := amount.Float64()
	payload := &models.TransactionCompleted{
		Hash:   transaction.Hash,
		Type:   transaction.Type,
		From:   transaction.From,
		To:     transaction.To,
		Amount: famount,
	}

	m.Notifier.Notify(ctx, &models.Notification{
		ClientID: transaction.To,
		Message:  payload,
	})
	m.Notifier.Notify(ctx, &models.Notification{
		ClientID: transaction.From,
		Message:  payload,
	})
}

func (m *Manager) IsWalletExist(ctx context.Context, from, to string) bool {
	cachedFrom, foundFrom := m.cache.Get(from)
	cachedTo, foundTo := m.cache.Get(to)

	// return cached value if both of the addresses found and both of the results are bool
	if foundFrom && foundTo {
		bcachedFrom, okFrom := cachedFrom.(bool)
		bcachedTo, okTo := cachedTo.(bool)

		if okFrom && okTo {
			return bcachedFrom || bcachedTo
		}
	}

	existingWallet := m.DB.ExistingWallet(ctx, from, to)
	if existingWallet == "" {
		m.cache.Set(from, false, cache.DefaultExpiration)
		m.cache.Set(to, false, cache.DefaultExpiration)
		return false
	}

	if existingWallet == from {
		m.cache.Set(to, false, cache.DefaultExpiration)
	} else {
		m.cache.Set(from, false, cache.DefaultExpiration)
	}

	m.cache.Set(existingWallet, true, cache.DefaultExpiration)
	return true
}

func (m *Manager) TxHasQueuedTransfers(ctx context.Context, txHash string) (string, error) {
	return m.DB.TxHasQueuedTransfers(ctx, txHash)
}

func (m *Manager) FailEthTransfer(ctx context.Context, id, msg string) error {
	return m.DB.FailEthTransfer(ctx, id, msg)
}

func (m *Manager) CompleteTransfer(ctx context.Context, transferHash string) error {
	return m.DB.CompleteTransfer(ctx, "", transferHash)
}

func (m *Manager) CompleteTransfers(ctx context.Context, ethTransferID string) error {
	return m.Transaction.CompleteTransfers(ctx, ethTransferID)
}

func (m *Manager) RegisterWallet(ctx context.Context, address string) {
	m.cache.Set(address, true, cache.DefaultExpiration)
}

func (m *Manager) GetPendingTopUps(ctx context.Context) ([]*models.DBTransfer, error) {
	dbTopUps, err := m.DB.GetPendingTopUps(ctx)
	if err != nil {
		return nil, err
	}

	var result []*models.DBTransfer
	for _, dbt := range dbTopUps {
		result = append(result, &models.DBTransfer{
			ID:          dbt.ID,
			TxHash:      dbt.GetTxHash(),
			FromAddress: dbt.FromAddress,
			ToAddress:   dbt.ToAddress,
			Amount:      dbt.Amount,
		})
	}
	return result, nil
}

func (m *Manager) GetPendingTransfers(ctx context.Context) ([]*models.DBTransfer, error) {
	dbTransfers, err := m.DB.GetPendingTransfers(ctx)
	if err != nil {
		return nil, err
	}

	var result []*models.DBTransfer
	for _, dbt := range dbTransfers {
		result = append(result, &models.DBTransfer{
			ID:          dbt.ID,
			TxHash:      dbt.GetTxHash(),
			FromAddress: dbt.FromAddress,
			ToAddress:   dbt.ToAddress,
			Amount:      dbt.Amount,
		})
	}
	return result, nil
}
