package scrappers

import (
	"context"
	"math/big"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/pkg/errors"

	"backend/app/config"
	"backend/app/models"
	"backend/app/scrapper"
	"backend/app/scrapper/token"
	"backend/app/storage/database"
	"backend/pkg/log"
)

const (
	monitorEventInterval = time.Minute
)

type EventsScraper struct {
	ethClient   *ethclient.Client
	token       *token.Token
	scrapperSvc scrapper.Service
	config      config.Ethereum
}

func NewEventsScraper(
	token *token.Token,
	ethClient *ethclient.Client,
	scrapperSvc scrapper.Service,
	config config.Ethereum,
) *EventsScraper {
	return &EventsScraper{
		ethClient:   ethClient,
		token:       token,
		scrapperSvc: scrapperSvc,
		config:      config,
	}
}

func (e *EventsScraper) Start() error {
	log.Info("Start event scrapper...")

	lastBlock, err := e.ethClient.BlockByNumber(context.TODO(), nil)
	if err != nil {
		return err
	}
	log.Debugf("Fetch the last block: %d", lastBlock.Number().Uint64())

	go e.extractOldEvents(lastBlock.Number().Uint64())
	go e.monitorEvents()
	return nil
}

func (e *EventsScraper) extractOldEvents(endBlockNumber uint64) {
	log.Info("Extract old events...")

	// fetch events by packets
	end := endBlockNumber
	start := end
	for start > 0 {
		start = end - e.config.PacketSize
		if start > end { // uint64 < 0
			start = 0
		}

		opts := new(bind.FilterOpts)
		opts.Start = start
		opts.End = &end
		transferIterator, err := e.token.FilterTransfer(opts, nil, nil)
		if err != nil {
			log.Errorf("Failed extract old events: %s", err)
			end = start
			continue
		}

		for it := transferIterator; it.Next(); {
			event := it.Event
			log.Debugf("TxHash: %s From: %s To: %s Value: %s", event.Raw.TxHash.Hex(), event.From.Hex(), event.To.Hex(), event.Value.String())
			newTransaction, needToSave := e.makeNewTransactionFromEvent(event, true)
			if needToSave {
				_, err := e.scrapperSvc.CreateTransaction(context.TODO(), newTransaction)
				if err != nil {
					log.Errorf("Error DB: %s", err)
				}
			}
		}

		end = start
	}

	log.Info("Extract old events... Done")
}

func (e *EventsScraper) monitorEvents() {
	for { // loop forever
		time.Sleep(monitorEventInterval)

		// fetch pending transfers
		pendingTransfers, err := e.scrapperSvc.GetPendingTransfers(context.TODO())
		if err != nil {
			log.Errorf("failed to get pending top ups: %s", err)
			continue
		}
		if len(pendingTransfers) < 1 {
			continue
		}

		wg := &sync.WaitGroup{}
		for _, pt := range pendingTransfers {
			go func(wg *sync.WaitGroup, pendingTransfer *models.DBTransfer) {
				wg.Add(1)
				defer wg.Done()

				_, isPending, err := e.ethClient.TransactionByHash(context.TODO(), common.HexToHash(pendingTransfer.TxHash))
				if err != nil {
					log.Debugf("failed to get tx by hash: %s", err)
					return
				}

				if isPending { // do not fail transfer because we have tx that is still in map pool here, just try later
					return
				}

				// save a transaction to the db
				newTx := &models.NewTransaction{
					Hash:   pendingTransfer.TxHash,
					Type:   database.TransferTypeTransferToken,
					Status: database.TransferStatusCompleted,
					Value:  pendingTransfer.Amount,
					From:   pendingTransfer.FromAddress,
					To:     pendingTransfer.ToAddress,
				}
				log.Infof("transfer %s completed", newTx.Hash)
				_, err = e.scrapperSvc.CreateTransaction(context.TODO(), newTx)
				if err != nil {
					log.Errorf("Error DB: %s", err)
				}
				if err = e.scrapperSvc.CompleteTransfer(context.TODO(), newTx.Hash); err != nil {
					log.Errorf("Error DB: %s", err)
				}
				e.scrapperSvc.NotifyClients(context.TODO(), newTx)
			}(wg, pt)
		}
		wg.Wait()
	}
}

func (e *EventsScraper) Listen(contractToken *token.Token, stop <-chan struct{}) error {
	log.Info("Start listen events...")
	eventsChan := make(chan *token.TokenTransfer)
	subscription, err := contractToken.WatchTransfer(&bind.WatchOpts{}, eventsChan, nil, nil)
	if err != nil {
		return err
	}
	errorChan := subscription.Err()
	for {
		select {
		case event := <-eventsChan:
			log.Debugf("TxHash: %s From: %s To: %s Value: %s", event.Raw.TxHash.Hex(), event.From.Hex(), event.To.Hex(), event.Value.String())
			newTransaction, needToSave := e.makeNewTransactionFromEvent(event, false)
			if needToSave {
				log.Infof("transfer %s completed", newTransaction.Hash)
				_, err := e.scrapperSvc.CreateTransaction(context.TODO(), newTransaction)
				if err != nil {
					log.Errorf("Error DB: %s", err)
				}
				if err = e.scrapperSvc.CompleteTransfer(context.TODO(), newTransaction.Hash); err != nil {
					log.Errorf("Error DB: %s", err)
				}
				e.scrapperSvc.NotifyClients(context.TODO(), newTransaction)
			}
		case err, ok := <-errorChan:
			if !ok || err == nil { // channel is closed
				subscription.Unsubscribe() // just in case
				return errors.New("events subscription error channel is closed")
			}
			log.Errorf("Event subscription error: %s", err)
		case <-stop:
			subscription.Unsubscribe() // just in case
			return nil
		}
	}
}

func (e *EventsScraper) makeNewTransactionFromEvent(event *token.TokenTransfer, fetchTime bool) (*models.NewTransaction, bool) {
	transaction := &models.NewTransaction{
		Hash:        event.Raw.TxHash.String(),
		BlockNumber: event.Raw.BlockNumber,
		From:        event.From.String(),
		To:          event.To.String(),
		Value:       event.Value.String(),
		Type:        database.TransferTypeTransferToken,
		Status:      database.TransferStatusCompleted,
	}

	needToSave := e.scrapperSvc.IsWalletExist(context.TODO(), transaction.From, transaction.To)

	if needToSave && fetchTime {
		block, err := e.ethClient.BlockByNumber(context.TODO(), new(big.Int).SetUint64(transaction.BlockNumber))
		if err != nil {
			log.Errorf("failed to get block by number %d: %s", transaction.BlockNumber, err)
		} else {
			transaction.Time = block.Time().Uint64()
		}
	}

	return transaction, needToSave
}
