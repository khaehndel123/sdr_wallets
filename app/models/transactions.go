package models

import (
	"strings"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/pkg/errors"

	"backend/pkg/eth"
)

const (
	TxStatusConfirmed = "confirmed"
	TxStatusFailed    = "failed"
)

type NewTransaction struct {
	Hash        string `json:"hash"`
	BlockNumber uint64 `json:"block_number"`
	Type        string `json:"type,omitempty"`
	Status      string `json:"status,omitempty"`
	From        string `json:"from,omitempty"`
	To          string `json:"to,omitempty"`
	Value       string `json:"value"`
	Time        uint64 `json:"time"`
}

func (t *NewTransaction) Validate() error {
	if t.Hash == "" {
		return errors.New("empty transaction hash provided")
	}

	return nil
}

type Transaction struct {
	Base
	NewTransaction
}

type NewTransfer struct {
	FromAddress string  `json:"-"` // filled from access token
	ToAddress   string  `json:"to_address,omitempty"`
	Amount      float64 `json:"amount,omitempty"`
}

func (t *NewTransfer) Validate() error {
	if t.FromAddress == "" {
		return errors.New("empty from address; it must be set on server during the processing, contact the support")
	}

	if t.ToAddress == "" {
		return errors.New("empty destination address provided")
	}

	if !eth.IsValidAddress(t.ToAddress) {
		return errors.New("invalid destination address provided")
	}

	// TODO: restore if needed
	//if t.Amount < 1 {
	//	return errors.New("1 SDR is the minimum amount allowed to transfer")
	//}

	return nil
}

type DBTransfer struct {
	ID          string `json:"id"`
	TxHash      string `json:"tx_hash"`
	FromAddress string `json:"from_address"`
	ToAddress   string `json:"to_address"`
	Amount      string `json:"amount"`
}

type TaxEstimation struct {
	Tax float64 `json:"tax"`
}

type UnsignedTransaction struct {
	Nonce    string `json:"nonce"`
	GasPrice string `json:"gas_price"`
	GasLimit string `json:"gas_limit"`
	To       string `json:"to"`
	Value    string `json:"value"`
	ChainID  string `json:"chain_id"`
	Data     string `json:"data"`
}

type UnsignedTransfer struct {
	ID           string               `json:"id"`
	TransferType string               `json:"transfer_type"`
	Amount       string               `json:"-"` // for internal needs
	ToAddress    string               `json:"-"` // for internal needs
	TxData       *UnsignedTransaction `json:"tx_data"`
}

type NonceValidator func(nonce uint64) error

type SignedTransaction struct {
	ID    string `json:"id,omitempty"`
	RawTx string `json:"raw_tx,omitempty"`
}

func (t *SignedTransaction) Validate(validateNonce NonceValidator) error {
	if t.ID == "" {
		return errors.New("empty id provided")
	}

	if t.RawTx == "" {
		return errors.New("empty raw transaction is provided")
	}

	// verify raw tx is valid
	var tx types.Transaction
	rawTxBytes, err := hexutil.Decode(t.RawTx)
	if err != nil {
		return errors.Wrap(err, "failed to decode raw tx into bytes")
	}
	if err = rlp.DecodeBytes(rawTxBytes, &tx); err != nil {
		return errors.Wrap(err, "failed to decode raw tx into a transaction")
	}
	if err = validateNonce(tx.Nonce()); err != nil {
		return errors.Wrap(err, "invalid nonce is provided")
	}

	return nil
}

type SignedTransfer struct {
	FromAddress  string               `json:"-"` // filled from access token
	Transactions []*SignedTransaction `json:"transactions,omitempty"`
}

func (t *SignedTransfer) Validate(validateNonce NonceValidator) error {
	if len(t.Transactions) != 2 {
		return errors.New("exactly two transactions are expected")
	}

	for _, t := range t.Transactions {
		if err := t.Validate(validateNonce); err != nil {
			return err
		}
	}

	return nil
}

type QueuedTransaction struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

type QueuedTransfer struct {
	Transactions []*QueuedTransaction `json:"transactions"`
}

type TransactionHistoryFilter struct {
	Address string  `json:"-"` // filled from access token
	After   uint64  `json:"-"` // filled from query param
	Skip    uint64  `json:"-"` // filled from query param
	Limit   *uint64 `json:"-"` // filled from query param
}

func (f *TransactionHistoryFilter) Validate() error {
	if f.Address == "" {
		return errors.New("empty address; it must be set on server during the processing, contact the support")
	}

	return nil
}

type TransactionHistoryItem struct {
	TxHash      string  `json:"tx_hash"`
	Status      string  `json:"status"`
	Amount      float64 `json:"amount"`
	FromAddress string  `json:"from_address"`
	ToAddress   string  `json:"to_address"`
	Time        float64 `json:"time"`
	Direction   string  `json:"direction"`
	Type        string  `json:"type"`
	EthFee      float64 `json:"eth_fee,omitempty"`
	SdrFee      float64 `json:"sdr_fee,omitempty"`
}

func (i *TransactionHistoryItem) IsCompleted() bool {
	return strings.HasPrefix(i.TxHash, "0x")
}

type ListMeta struct {
	Total uint64 `json:"total"`
}

type TransactionHistory struct {
	Transactions []*TransactionHistoryItem `json:"transactions"`
	Meta         *ListMeta                 `json:"meta"`
}

type TransactionFilter struct {
	Address string `json:"-"` // filled from access token
	Hash    string `json:"-"` // filled from path param
}

func (f *TransactionFilter) Validate() error {
	if f.Address == "" {
		return errors.New("empty address; it must be set on server during the processing, contact the support")
	}
	if f.Hash == "" {
		return errors.New("empty tx hash is provided")
	}
	return nil
}
