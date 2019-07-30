package scrappers

import (
	"context"
	"math/big"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/pkg/errors"

	"backend/app/config"
	"backend/app/models"
	"backend/app/scrapper"
	"backend/app/storage/database"
	"backend/pkg/log"
)

const (
	monitorTopUpInterval = time.Minute
)

type TxScraper struct {
	ethClient   *ethclient.Client
	scrapperSvc scrapper.Service
	config      config.Ethereum
	contractABI abi.ABI
}

func NewTxScraper(
	ethClient *ethclient.Client,
	scrapperSvc scrapper.Service,
	config config.Ethereum,
	contractABI abi.ABI,
) *TxScraper {
	return &TxScraper{
		ethClient:   ethClient,
		scrapperSvc: scrapperSvc,
		config:      config,
		contractABI: contractABI,
	}
}

func (t *TxScraper) Start() error {
	log.Info("Start transaction scrapper...")

	lastBlock, err := t.ethClient.BlockByNumber(context.TODO(), nil)
	if err != nil {
		return err
	}
	log.Debugf("Fetch the last block: %d", lastBlock.Number().Uint64())

	go t.extractOldTransactions(lastBlock.Number().Uint64())
	go t.monitorTopUpTransactions() // fallback in case if listener didn't catch a new block
	return nil
}

func (t *TxScraper) extractOldTransactions(endBlockNumber uint64) {
	log.Info("Extract old transactions...")
	for i := endBlockNumber; i >= 0; i-- {
		block, err := t.ethClient.BlockByNumber(context.TODO(), new(big.Int).SetUint64(i))
		if err != nil {
			log.Errorf("Failed get block by number %d: %s", i, err)
			continue
		}
		t.extractTransactionsFromBlock(block)
	}
	log.Info("Extract old transactions... Done")
}

// monitorTopUpTransactions is a fallback for listen function
func (t *TxScraper) monitorTopUpTransactions() {
	for { // loop forever
		time.Sleep(monitorTopUpInterval)

		// fetch pending ETH top up transfers
		pendingTopUps, err := t.scrapperSvc.GetPendingTopUps(context.TODO())
		if err != nil {
			log.Errorf("failed to get pending top ups: %s", err)
			continue
		}
		if len(pendingTopUps) < 1 {
			continue
		}

		wg := &sync.WaitGroup{}
		for _, pt := range pendingTopUps {
			go func(wg *sync.WaitGroup, pendingTopUp *models.DBTransfer) {
				wg.Add(1)
				defer wg.Done()

				_, isPending, err := t.ethClient.TransactionByHash(context.TODO(), common.HexToHash(pendingTopUp.TxHash))
				if err != nil {
					log.Debugf("failed to get tx by hash: %s", err)
					return
				}

				if isPending { // do not fail transfer because we have tx that is still in map pool here, just try later
					return
				}

				// check their statuses
				rcpt, err := t.ethClient.TransactionReceipt(context.TODO(), common.HexToHash(pendingTopUp.TxHash))
				if err != nil || rcpt == nil {
					log.Errorf("error getting transaction receipt: %s", err)
					return
				}

				// complete a transfer if needed
				if rcpt.Status > 0 { // if success
					log.Infof("eth transfer %s confirmed", pendingTopUp.ID)
					go t.completeTransfer(context.TODO(), pendingTopUp.ID)
				} else {
					log.Errorf("eth transfer %s, tx hash %s failed", pendingTopUp.ID, pendingTopUp.TxHash)
					go t.failTransfer(context.TODO(), pendingTopUp.ID, "ETH top-up transaction failed")
				}

				// save a transaction to the db
				newTx := &models.NewTransaction{
					Hash:   pendingTopUp.TxHash,
					Type:   database.TransferTypeTxETH,
					Status: database.TransferStatusCompleted,
					Value:  pendingTopUp.Amount,
					From:   pendingTopUp.FromAddress,
					To:     pendingTopUp.ToAddress,
				}
				_, err = t.scrapperSvc.CreateTransaction(context.TODO(), newTx)
				if err != nil {
					log.Errorf("Error DB: %s", err)
				}
			}(wg, pt)
		}
		wg.Wait()
	}
}

func (t *TxScraper) Listen(ethClient *ethclient.Client, stop <-chan struct{}) error {
	log.Info("Start listen transactions...")
	headersChan := make(chan *types.Header)
	subscription, err := ethClient.SubscribeNewHead(context.TODO(), headersChan)
	if err != nil {
		return err
	}
	errorChan := subscription.Err()
	for {
		select {
		case header := <-headersChan:
			block, err := t.ethClient.BlockByHash(context.TODO(), header.Hash())
			if err != nil {
				log.Debugf("Failed get block by hash %s: %s", header.Hash().String(), err)
				continue
			}
			t.extractTransactionsFromBlock(block)
		case err, ok := <-errorChan:
			if !ok || err == nil { // channel is closed
				subscription.Unsubscribe() // just in case
				return errors.New("headers subscription error channel is closed")
			}
			log.Errorf("Headers subscription error: %s", err)
		case <-stop:
			subscription.Unsubscribe() // just in case
			return nil
		}
	}
}

func (t *TxScraper) extractTransactionsFromBlock(block *types.Block) {
	log.Debugf("Processing block: height = %s txs=%d", block.Number().String(), len(block.Transactions()))
	for _, tx := range block.Transactions() {
		log.Debugf("Processing tx: hash = %s", tx.Hash().Hex())
		newTransaction, needToSave := t.makeNewTransactionFromTx(block, tx)
		if needToSave {
			_, err := t.scrapperSvc.CreateTransaction(context.TODO(), newTransaction)
			if err != nil {
				log.Errorf("Error DB: %s", err)
			}
		}
	}
}

func (t *TxScraper) makeNewTransactionFromTx(block *types.Block, tx *types.Transaction) (*models.NewTransaction, bool) {
	transaction := &models.NewTransaction{
		Hash:        tx.Hash().String(),
		BlockNumber: block.NumberU64(),
		Value:       tx.Value().String(),
		Time:        block.Time().Uint64(),
		Type:        database.TransferTypeTxETH,
		Status:      database.TransferStatusCompleted,
	}
	from, err := extractFrom(tx)
	if err != nil {
		log.Errorf("Failed extract \"from\" from tx: %s error: %s", tx.Hash().Hex(), err)
	}
	if from != nil {
		transaction.From = from.String()
	}
	if tx.To() != nil {
		transaction.To = tx.To().String()
	}
	needToSave := t.scrapperSvc.IsWalletExist(context.TODO(), transaction.From, transaction.To)
	if !needToSave {
		return transaction, needToSave
	}

	// check if the transaction is one we are waiting for to transfer tokens
	transferID, err := t.scrapperSvc.TxHasQueuedTransfers(context.TODO(), tx.Hash().String())
	if err != nil {
		transaction := t.makeTokenTransferIfNeeded(transaction)
		return transaction, needToSave
	}

	// complete tx if needed
	go func(tx *types.Transaction) {
		// check transaction status
		rcpt, err := t.ethClient.TransactionReceipt(context.TODO(), tx.Hash())
		if err == nil && rcpt != nil && rcpt.Status < 1 { // if a receipt exists and tx failed
			log.Errorf("eth transfer %s, tx hash %s failed", transferID, tx.Hash().String())
			t.failTransfer(context.TODO(), transferID, "ETH top-up transaction failed")
			return
		}
		// else we don't care - complete the transfers
		log.Infof("eth transfer %s confirmed", transferID)
		t.completeTransfer(context.TODO(), transferID)
	}(tx)
	return transaction, needToSave
}

func (t *TxScraper) completeTransfer(ctx context.Context, ethTransferID string) {
	if err := t.scrapperSvc.CompleteTransfers(ctx, ethTransferID); err != nil {
		log.Errorf("failed to complete the transfers associated with eth transfer %s: %s", ethTransferID, err)
	}
}

func (t *TxScraper) failTransfer(ctx context.Context, ethTransferID, message string) {
	if err := t.scrapperSvc.FailEthTransfer(ctx, ethTransferID, message); err != nil {
		log.Errorf("Error writing transfer error message %s", err)
	}
}

func (t *TxScraper) makeTokenTransferIfNeeded(tx *models.NewTransaction) *models.NewTransaction {
	// if a token transfer
	if (strings.EqualFold(tx.From, t.config.TokenAddress) || strings.EqualFold(tx.To, t.config.TokenAddress)) &&
		(tx.Value == "0" || tx.Value == "") {
		rcpt, err := t.ethClient.TransactionReceipt(context.TODO(), common.HexToHash(tx.Hash))
		if err != nil {
			log.Debugf("error getting transaction receipt: %s", err)
			return tx
		}
		if len(rcpt.Logs) < 1 {
			return tx
		}

		logs := rcpt.Logs[0]
		if len(logs.Topics) < 3 {
			return tx
		}

		// extract amount
		var transferEvent struct {
			From  common.Address
			To    common.Address
			Value *big.Int
		}
		if err = t.contractABI.Unpack(&transferEvent, "Transfer", logs.Data); err != nil {
			log.Debugf("failed to unpack event: %s", err)
			return tx
		}

		// set amount, from and to addresses
		tx.Value = transferEvent.Value.String()
		tx.From = common.BytesToAddress(logs.Topics[1].Bytes()).String()
		tx.To = common.BytesToAddress(logs.Topics[2].Bytes()).String()

		tx.Type = database.TransferTypeTransferToken
	}
	return tx
}

func extractFrom(tx *types.Transaction) (*common.Address, error) {
	V, _, _ := tx.RawSignatureValues()
	if V != nil {
		signer := deriveSigner(V, tx.Protected())
		sender, err := types.Sender(signer, tx)
		if err != nil {
			return nil, err
		}
		return &sender, nil
	}
	return nil, nil
}

func deriveSigner(V *big.Int, isProtected bool) types.Signer {
	if V.Sign() != 0 && isProtected {
		return types.NewEIP155Signer(deriveChainId(V))
	}
	return types.HomesteadSigner{}
}

func deriveChainId(v *big.Int) *big.Int {
	if v.BitLen() <= 64 {
		v := v.Uint64()
		if v == 27 || v == 28 {
			return new(big.Int)
		}
		return new(big.Int).SetUint64((v - 35) / 2)
	}
	v = new(big.Int).Sub(v, big.NewInt(35))
	return v.Div(v, big.NewInt(2))
}
