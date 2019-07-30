package database

import (
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common/hexutil"

	"backend/app/models"
	"backend/pkg/eth"
)

const (
	// transfer types
	TransferTypeTransferToken = "transfer_token"
	TransferTypeTxETH         = "ethereum_tx"
	TransferTypeTax           = "tax"
	TransferTypeEthTopUp      = "ethtopup"

	// transfer statuses
	TransferStatusNew        = "new"
	TransferStatusQueued     = "queued"
	TransferStatusPending    = "pending"
	TransferStatusFailed     = "failed"
	TransferStatusCompleted  = "completed"
	TransferStatusProcessing = "processing"
	TransferStatusConfirming = "confirming"
)

type Base struct {
	ID        string     `db:"id"`
	CreatedAt time.Time  `db:"created_at"`
	UpdatedAt *time.Time `db:"updated_at"`
	DeletedAt *time.Time `db:"deleted_at"`
}

func (b *Base) GetUpdatedAtUnix() int64 {
	if b == nil || b.UpdatedAt == nil {
		return 0
	}
	return b.UpdatedAt.Unix()
}

func (b *Base) GetDeletedAtUnix() int64 {
	if b == nil || b.DeletedAt == nil {
		return 0
	}
	return b.DeletedAt.Unix()
}

func (b *Base) ToPublic() models.Base {
	return models.Base{
		ID:        b.ID,
		CreatedAt: b.CreatedAt.Unix(),
		UpdatedAt: b.GetUpdatedAtUnix(),
	}
}

type NewWallet struct {
	Address     string    `db:"address"`
	GeneratedAt time.Time `db:"generated_at"`
}

func NewWalletFromPublic(wallet *models.NewWallet) *NewWallet {
	return &NewWallet{
		Address:     wallet.Address,
		GeneratedAt: time.Unix(int64(wallet.GeneratedAt), 0),
	}
}

type Wallet struct {
	Base
	NewWallet
}

func (w *Wallet) ToPublic() *models.Wallet {
	return &models.Wallet{
		Base: w.Base.ToPublic(),
		NewWallet: models.NewWallet{
			Address:     w.Address,
			GeneratedAt: w.GeneratedAt.Unix(),
		},
	}
}

type NewTransaction struct {
	Hash        string `db:"hash"`
	BlockNumber uint64 `db:"block_number"`
	Type        string `db:"type"`
	Status      string `db:"status"`
	From        string `db:"from_address"`
	To          string `db:"to_address"`
	Value       string `db:"value"`
	Time        uint64 `db:"time"`
}

func NewTransactionFromPublic(transaction *models.NewTransaction) *NewTransaction {
	return &NewTransaction{
		Hash:        transaction.Hash,
		BlockNumber: transaction.BlockNumber,
		Type:        transaction.Type,
		Status:      transaction.Status,
		From:        transaction.From,
		To:          transaction.To,
		Value:       transaction.Value,
		Time:        transaction.Time,
	}
}

type Transaction struct {
	Base
	NewTransaction
}

func (t *Transaction) ToPublic() *models.Transaction {
	return &models.Transaction{
		Base: t.Base.ToPublic(),
		NewTransaction: models.NewTransaction{
			Hash:        t.Hash,
			BlockNumber: t.BlockNumber,
			Type:        t.Type,
			Status:      t.Status,
			From:        t.From,
			To:          t.To,
			Value:       t.Value,
			Time:        t.Time,
		},
	}
}

type NewTransfer struct {
	TransferType string `db:"transfer_type"`
	FromAddress  string `db:"from_address"`
	Nonce        uint64 `db:"nonce"`
	GasPrice     string `db:"gas_price"`
	GasLimit     uint64 `db:"gas_limit"`
	Amount       string `db:"amount"`
	ToAddress    string `db:"to_address"`
	TaxAmount    string `db:"tax_amount"`
}

func NewTransferFromPublic(transfer *models.UnsignedTransfer, fromAddress, taxAmount string) *NewTransfer {
	nonce, _ := hexutil.DecodeUint64(transfer.TxData.Nonce)
	gasPrice, _ := hexutil.DecodeBig(transfer.TxData.GasPrice)
	gasLimit, _ := hexutil.DecodeUint64(transfer.TxData.GasLimit)
	return &NewTransfer{
		TransferType: transfer.TransferType,
		FromAddress:  fromAddress,
		Nonce:        nonce,
		GasPrice:     gasPrice.String(),
		GasLimit:     gasLimit,
		Amount:       transfer.Amount,
		ToAddress:    transfer.ToAddress,
		TaxAmount:    taxAmount,
	}
}

type Transfer struct {
	Base
	NewTransfer
	EthTransferID *string `db:"eth_transfer_id"`
	TxHash        *string `db:"tx_hash"`
	RawTx         *string `db:"raw_tx"`
	Status        string  `db:"status"`
	Message       string  `db:"message"`
}

func (t *Transfer) IsProcessing() bool {
	return !(t.Status == TransferStatusNew || t.Status == TransferStatusConfirming)
}

func (t *Transfer) GetEthTransferID() string {
	if t == nil || t.EthTransferID == nil {
		return ""
	}
	return *t.EthTransferID
}

func (t *Transfer) GetTxHash() string {
	if t == nil || t.TxHash == nil {
		return ""
	}
	return *t.TxHash
}

func (t *Transfer) GetRawTx() string {
	if t == nil || t.RawTx == nil {
		return ""
	}
	return *t.RawTx
}

type TransactionHistoryFilter struct {
	Address     string
	After       uint64
	BankAddress string
	Skip        uint64
	Limit       *uint64
}

type TransactionFilter struct {
	Address string
	Hash    string
}

type TransactionHistoryItem struct {
	TxHash      string   `db:"tx_hash"`
	Status      string   `db:"status"`
	Amount      string   `db:"amount"`
	FromAddress string   `db:"from_address"`
	ToAddress   string   `db:"to_address"`
	Time        *float64 `db:"time"`
	Direction   string   `db:"direction"`
	Type        string   `db:"type"`
}

func (i *TransactionHistoryItem) GetTime() float64 {
	if i == nil || i.Time == nil {
		return 0
	}
	return *i.Time
}

func (i *TransactionHistoryItem) DetectDirection(address string) {
	if strings.EqualFold(i.ToAddress, address) {
		i.Direction = "in"
		return
	}
	i.Direction = "out"
}

func (i *TransactionHistoryItem) ToPublic(decimals uint8) *models.TransactionHistoryItem {
	damount := eth.ToETH(i.Amount, decimals)
	amount, _ := damount.Float64()

	return &models.TransactionHistoryItem{
		TxHash:      i.TxHash,
		Status:      i.Status,
		Amount:      amount,
		FromAddress: i.FromAddress,
		ToAddress:   i.ToAddress,
		Time:        i.GetTime(),
		Direction:   i.Direction,
		Type:        i.Type,
	}
}
