package database

import (
	"context"
	"time"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres" // also imports "github.com/lib/pq"
	"github.com/golang-migrate/migrate/v4/source/go_bindata"
	"github.com/jmoiron/sqlx"
	"github.com/pkg/errors"
	"go.uber.org/multierr"

	"backend/app/storage/migrations"
	"backend/pkg/uuid"
)

type Postgres struct {
	DB *sqlx.DB
}

func Connect(cfg Config) (*Postgres, error) {
	connectionString := cfg.DBConnectionString()
	db, err := sqlx.Connect("postgres", connectionString)
	if err != nil {
		return nil, errors.Wrap(err, "failed to connect to the database")
	}

	// auto-migrate the db
	if err = migrateDB(cfg); err != nil {
		return nil, errors.Wrap(err, "failed to migrate the database")
	}

	pg := &Postgres{DB: db}
	return pg, nil
}

func (p *Postgres) Close() error {
	return p.DB.Close()
}

func (p *Postgres) CreateWallet(ctx context.Context, wallet *NewWallet) (*Wallet, error) {
	id := uuid.NewUUID()
	result := &Wallet{
		Base: Base{
			ID:        id,
			CreatedAt: time.Now(),
		},
		NewWallet: *wallet,
	}

	_, err := p.DB.NamedExecContext(
		ctx,
		"INSERT INTO wallets (id, address, generated_at, created_at) VALUES (:id, :address, :generated_at, :created_at);",
		result,
	)
	if err != nil {
		return nil, errors.Wrap(err, "failed to insert a wallet")
	}
	return result, nil
}

func (p *Postgres) GetWallet(ctx context.Context, address string) (*Wallet, error) {
	result := new(Wallet)
	if err := p.DB.GetContext(
		ctx,
		result,
		"SELECT * FROM wallets WHERE LOWER(address) = LOWER($1) AND deleted_at IS NULL LIMIT 1;",
		address,
	); err != nil {
		return nil, errors.Wrap(err, "failed to select a wallet")
	}
	return result, nil
}

func (p *Postgres) ExistingWallet(ctx context.Context, from, to string) string {
	var result string
	if err := p.DB.GetContext(
		ctx,
		&result,
		"SELECT address FROM wallets WHERE (LOWER(address) = LOWER($1) OR LOWER(address) = LOWER($2) ) AND deleted_at IS NULL LIMIT 1;",
		from, to,
	); err != nil {
		return ""
	}
	return result
}

func (p *Postgres) TxHasQueuedTransfers(ctx context.Context, txHash string) (string, error) {
	var result string
	if err := p.DB.GetContext(
		ctx,
		&result,
		"SELECT id FROM transfers WHERE LOWER(tx_hash) = LOWER($1) AND transfer_type = $2 AND status = $3 AND deleted_at IS NULL LIMIT 1;",
		txHash, TransferTypeEthTopUp, TransferStatusPending,
	); err != nil {
		return "", errors.Wrap(err, "failed to find an eth transfer by hash")
	}
	return result, nil
}

func (p *Postgres) CreateTransaction(ctx context.Context, transaction *NewTransaction) (*Transaction, error) {
	id := uuid.NewUUID()
	result := &Transaction{
		Base: Base{
			ID:        id,
			CreatedAt: time.Now(),
		},
		NewTransaction: *transaction,
	}
	query := `INSERT INTO transactions (id, block_number, hash, type, status, from_address, to_address, value, time, created_at) 
				VALUES (:id, :block_number, :hash, :type, :status, :from_address, :to_address, :value, :time, :created_at) `
	if result.Time != 0 { // update time only
		query += "ON CONFLICT (hash) DO UPDATE SET time = EXCLUDED.time, updated_at = NOW();"
	} else {
		query += "ON CONFLICT (hash) DO NOTHING;"
	}
	_, err := p.DB.NamedExecContext(
		ctx,
		query,
		result,
	)
	if err != nil {
		return nil, errors.Wrap(err, "failed to insert a transaction")
	}
	return result, nil
}

func (p *Postgres) CreateTransfers(ctx context.Context, transfers []*NewTransfer) ([]*Transfer, error) {
	// set the ids
	var result []*Transfer
	for _, t := range transfers {
		result = append(result, &Transfer{
			Base: Base{
				ID:        uuid.NewUUID(),
				CreatedAt: time.Now(),
			},
			NewTransfer: *t,
			Status:      TransferStatusNew,
		})
	}

	tx, err := p.DB.Beginx()
	if err != nil {
		return nil, errors.Wrap(err, "failed to start a db transaction")
	}

	for _, t := range result {
		_, err := tx.NamedExecContext(
			ctx,
			"INSERT INTO transfers (id, transfer_type, from_address, nonce, gas_price, gas_limit, status, created_at, amount, to_address, tax_amount) VALUES (:id, :transfer_type, :from_address, :nonce, :gas_price, :gas_limit, :status, :created_at, :amount, :to_address, :tax_amount);",
			t,
		)
		if err != nil {
			err = errors.Wrap(err, "failed to insert a transfer")
			rlbErr := errors.Wrap(tx.Rollback(), "failed to rollback the db transaction")
			return nil, multierr.Append(err, rlbErr)
		}
	}

	if err = tx.Commit(); err != nil {
		return nil, errors.Wrap(err, "failed to commit the db transaction")
	}
	return result, nil
}

func (p *Postgres) GetAndConfirmTransferByID(ctx context.Context, id, from string) (*Transfer, error) {
	result := new(Transfer)
	if _, err := p.DB.ExecContext(
		ctx,
		"UPDATE transfers SET status = $1 WHERE id = $2 AND LOWER(from_address) = LOWER($3) AND deleted_at IS NULL;",
		TransferStatusConfirming, id, from,
	); err != nil {
		return nil, errors.Wrap(err, "failed to set confirming status to a transfer")
	}
	if err := p.DB.GetContext(
		ctx,
		result,
		"SELECT * FROM transfers WHERE id = $1 AND LOWER(from_address) = LOWER($2) AND deleted_at IS NULL LIMIT 1;",
		id, from,
	); err != nil {
		return nil, errors.Wrap(err, "failed to select a transfer by id")
	}
	return result, nil
}

func (p *Postgres) GetTransferByIDOrHash(ctx context.Context, id, hash string) (*Transfer, error) {
	result := new(Transfer)
	if err := p.DB.GetContext(
		ctx,
		result,
		"SELECT * FROM transfers WHERE id = $1 OR LOWER(tx_hash) = LOWER($2) AND deleted_at IS NULL LIMIT 1;",
		id, hash,
	); err != nil {
		return nil, errors.Wrap(err, "failed to select a transfer by id or hash")
	}
	return result, nil
}

func (p *Postgres) UpdateTransfers(ctx context.Context, transfers []*Transfer) ([]*Transfer, error) {
	tx, err := p.DB.Beginx()
	if err != nil {
		return nil, errors.Wrap(err, "failed to start a db transaction")
	}

	now := time.Now()
	for _, t := range transfers {
		t.UpdatedAt = &now
		_, err := tx.NamedExecContext(
			ctx,
			"UPDATE transfers SET tx_hash = :tx_hash, raw_tx = :raw_tx, status = :status, eth_transfer_id = :eth_transfer_id, updated_at = :updated_at WHERE id = :id AND deleted_at IS NULL;",
			t,
		)
		if err != nil {
			err = errors.Wrap(err, "failed to update a transfer")
			rlbErr := errors.Wrap(tx.Rollback(), "failed to rollback the db transaction")
			return nil, multierr.Append(err, rlbErr)
		}
	}

	if err = tx.Commit(); err != nil {
		return nil, errors.Wrap(err, "failed to commit the db transaction")
	}
	return transfers, nil
}

func (p *Postgres) CreateEthTransfer(ctx context.Context, transfer *NewTransfer, txHash string) (*Transfer, error) {
	id := uuid.NewUUID()
	result := &Transfer{
		Base: Base{
			ID:        id,
			CreatedAt: time.Now(),
		},
		NewTransfer: *transfer,
		TxHash:      &txHash,
		Status:      TransferStatusPending,
	}

	_, err := p.DB.NamedExecContext(
		ctx,
		"INSERT INTO transfers (id, transfer_type, from_address, nonce, gas_price, gas_limit, status, tx_hash, created_at) VALUES (:id, :transfer_type, :from_address, :nonce, :gas_price, :gas_limit, :status, :tx_hash, :created_at);",
		result,
	)
	if err != nil {
		return nil, errors.Wrap(err, "failed to insert an eth transfer")
	}
	return result, nil
}

func (p *Postgres) CompleteTransfer(ctx context.Context, id, hash string) error {
	_, err := p.DB.ExecContext(
		ctx,
		"UPDATE transfers SET status = $1, updated_at = NOW() WHERE (id = $2 OR LOWER(tx_hash) = LOWER($3)) AND deleted_at IS NULL;",
		TransferStatusCompleted, id, hash,
	)
	return errors.Wrap(err, "failed to set completion status for a transfer")
}

func (p *Postgres) StartEthTransferProcessing(ctx context.Context, id string) error {
	_, err := p.DB.ExecContext(
		ctx,
		"UPDATE transfers SET status = $1, updated_at = $2 WHERE id = $3 AND deleted_at IS NULL;",
		TransferStatusProcessing, time.Now(), id,
	)
	return errors.Wrap(err, "failed to set processing status for a top up transfer")
}

func (p *Postgres) FailEthTransfer(ctx context.Context, id, msg string) error {
	if err := p.WriteTransferError(ctx, id, msg); err != nil {
		return err
	}
	_, err := p.DB.ExecContext(
		ctx,
		"UPDATE transfers SET message = $1, status = $2, updated_at = $3 WHERE eth_transfer_id = $4 AND deleted_at IS NULL;",
		msg, TransferStatusFailed, time.Now(), id,
	)
	return errors.Wrap(err, "failed to mark linked transfers as failed")
}

func (p *Postgres) WriteTransferError(ctx context.Context, id, msg string) error {
	_, err := p.DB.ExecContext(
		ctx,
		"UPDATE transfers SET message = $1, status = $2, updated_at = $3 WHERE id = $4 AND deleted_at IS NULL;",
		msg, TransferStatusFailed, time.Now(), id,
	)
	if err != nil {
		return errors.Wrap(err, "failed to write the error message to a transfer")
	}
	return nil
}

func (p *Postgres) GetQueuedTransfersFor(ctx context.Context, ethTransferID string) ([]*Transfer, error) {
	var result []*Transfer
	if err := p.DB.SelectContext(
		ctx,
		&result,
		"SELECT * FROM transfers WHERE eth_transfer_id = $1 AND status = $2 AND deleted_at IS NULL ORDER BY nonce ASC;",
		ethTransferID, TransferStatusQueued,
	); err != nil {
		return nil, errors.Wrap(err, "failed to select queued transfers")
	}
	return result, nil
}

func (p *Postgres) IsNoncePending(ctx context.Context, nonce uint64, address string) bool {
	var tmp uint64
	if err := p.DB.GetContext(
		ctx,
		&tmp,
		"SELECT nonce FROM transfers WHERE nonce = $1 AND LOWER(from_address) = LOWER($2) AND (status = $3 OR status = $4) AND deleted_at IS NULL LIMIT 1;",
		nonce, address, TransferStatusQueued, TransferStatusPending,
	); err != nil {
		return false
	}
	return true
}

func (p *Postgres) TransactionHistory(ctx context.Context, filter *TransactionHistoryFilter) (
	[]*TransactionHistoryItem, uint64, error,
) {
	var result []*TransactionHistoryItem
	if err := p.DB.SelectContext(
		ctx,
		&result,
		queryTxHistoryPaginated,
		filter.Address, filter.After, filter.BankAddress, filter.Skip, filter.Limit,
	); err != nil {
		return nil, 0, errors.Wrap(err, "failed to select transaction history")
	}

	var total uint64
	if err := p.DB.GetContext(
		ctx,
		&total,
		queryTxHistoryCount,
		filter.Address, filter.After, filter.BankAddress,
	); err != nil {
		return nil, 0, errors.Wrap(err, "failed to count transaction history")
	}
	return result, total, nil
}

func (p *Postgres) GetTransactionOrTransfer(ctx context.Context, filter *TransactionFilter) (
	*TransactionHistoryItem, error,
) {
	// try to fetch tx from the transactions table first
	result := new(TransactionHistoryItem)
	if err := p.DB.GetContext(
		ctx,
		result,
		"SELECT hash tx_hash, status, value amount, from_address, to_address, COALESCE(NULLIF(time, 0), EXTRACT(epoch from created_at)::bigint) AS time, type FROM transactions WHERE (LOWER(hash) = LOWER($1) OR LOWER(id) = LOWER($1)) AND (LOWER(from_address) = LOWER($2) OR LOWER(to_address) = LOWER($2)) AND deleted_at IS NULL LIMIT 1;",
		filter.Hash, filter.Address,
	); err == nil {
		result.DetectDirection(filter.Address)
		return result, nil
	}

	// then try to load tx from transfers
	if err := p.DB.GetContext(
		ctx,
		result,
		"SELECT id tx_hash, status, amount, from_address, to_address, EXTRACT(epoch from created_at)::bigint AS time, 'out' direction, transfer_type as type FROM transfers WHERE (LOWER(id) = LOWER($1) OR LOWER(tx_hash) = LOWER($1)) AND LOWER(from_address) = LOWER($2) AND deleted_at IS NULL LIMIT 1;",
		filter.Hash, filter.Address,
	); err != nil {
		return nil, errors.Wrap(err, "failed to get a transaction from the db")
	}
	return result, nil
}

func (p *Postgres) GetPendingTopUps(ctx context.Context) ([]*Transfer, error) {
	var result []*Transfer
	if err := p.DB.SelectContext(
		ctx,
		&result,
		"SELECT * FROM transfers WHERE transfer_type = $1 AND status = $2 AND deleted_at IS NULL;",
		TransferTypeEthTopUp, TransferStatusPending,
	); err != nil {
		return nil, errors.Wrap(err, "failed to select pending eth top ups")
	}
	return result, nil
}

func (p *Postgres) GetPendingTransfers(ctx context.Context) ([]*Transfer, error) {
	var result []*Transfer
	if err := p.DB.SelectContext(
		ctx,
		&result,
		"SELECT * FROM transfers WHERE transfer_type <> $1 AND status = $2 AND deleted_at IS NULL;",
		TransferTypeEthTopUp, TransferStatusPending,
	); err != nil {
		return nil, errors.Wrap(err, "failed to select pending eth top ups")
	}
	return result, nil
}

func (p *Postgres) ClearUnsignedTransfers(ctx context.Context, address string) error {
	_, err := p.DB.ExecContext(
		ctx,
		"UPDATE transfers SET deleted_at = NOW() WHERE LOWER(from_address) = LOWER($1) AND status = $2;",
		address, TransferStatusNew,
	)
	return errors.Wrap(err, "failed to set deleted_at for unsigned transfers")
}

func migrateDB(cfg Config) error {
	res := bindata.Resource(migrations.AssetNames(), func(name string) ([]byte, error) {
		return migrations.Asset(name)
	})
	driver, err := bindata.WithInstance(res)
	if err != nil {
		return errors.WithMessage(err, "failed to initialize a driver")
	}

	connectionString := cfg.DBConnectionStringForMigration()
	migration, err := migrate.NewWithSourceInstance("go-bindata", driver, connectionString)
	if err != nil {
		return errors.WithMessage(err, "failed to initialize a migration instance")
	}

	err = migration.Up()
	if err != nil && err.Error() == "no change" { // "no change" is not an error
		err = nil
	}
	return errors.WithMessage(err, "failed to execute migrations")
}
