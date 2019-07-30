package transaction

import (
	"context"
	"crypto/ecdsa"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/pkg/errors"
	"github.com/shopspring/decimal"
	"golang.org/x/crypto/sha3"

	"backend/app/config"
	"backend/app/models"
	"backend/app/scrapper/token"
	"backend/app/sdrsidecar"
	"backend/app/storage/database"
	"backend/pkg/eth"
	"backend/pkg/log"
)

const (
	ethTxGasLimit = 21000
)

var (
	transferMethodID []byte
)

func init() {
	transferFnSignature := []byte("transfer(address,uint256)")
	hash := sha3.NewLegacyKeccak256()
	hash.Write(transferFnSignature)
	transferMethodID = hash.Sum(nil)[:4]
}

type Manager struct {
	TxConfig   config.Transaction
	EthConfig  config.Ethereum
	DB         database.Database
	EthClient  *ethclient.Client
	RpcClient  *rpc.Client
	SdrToken   *token.Token
	SdrSidecar sdrsidecar.Service

	tokenDecimals uint8 // cached version
}

// EstimateTax calculates gas fee for all three transactions and
// adds a little bit to tax gets higher than expenses.
func (m *Manager) EstimateTax(ctx context.Context, tx *models.NewTransfer) (*models.TaxEstimation, error) {
	log.AddFields(ctx, "tx", tx)

	if err := tx.Validate(); err != nil {
		return nil, err
	}

	tax, _, err := m.estimateTax(ctx, tx.Amount)
	if err != nil {
		return nil, err
	}
	return &models.TaxEstimation{Tax: tax}, nil
}

func (m *Manager) CreateTransfer(ctx context.Context, tx *models.NewTransfer) (
	[]*models.UnsignedTransfer, error,
) {
	log.AddFields(ctx, "tx", tx)

	if err := tx.Validate(); err != nil {
		return nil, err
	}

	// clear current new transactions for the address
	if err := m.DB.ClearUnsignedTransfers(ctx, tx.FromAddress); err != nil {
		return nil, errors.Wrap(err, "failed to clear previously created unsigned transactions")
	}

	tax, gasPrice, err := m.estimateTax(ctx, tx.Amount)
	if err != nil {
		return nil, errors.Wrap(err, "failed to calculate a tax")
	}

	// ensure user has enough tokens to complete the transfer
	balance, err := m.SdrToken.BalanceOf(&bind.CallOpts{Context: ctx}, common.HexToAddress(tx.FromAddress))
	if err != nil {
		return nil, errors.Wrap(err, "failed to check token balance")
	}
	decimals, err := m.getTokenDecimals(ctx)
	if err != nil {
		return nil, err
	}
	expenses := eth.ToWei(tx.Amount+tax, decimals)
	if balance.Cmp(expenses) < 0 {
		return nil, errors.New("you have no enough tokens to complete the transfer")
	}

	// chainID
	chainID, err := m.EthClient.NetworkID(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to retrieve chain id")
	}

	// create two transactions: first one to transfer tokens to a destination address,
	// and the second one transferring tax to a bank address
	transferTx, nonce, err := m.createTransfer(
		ctx,
		database.TransferTypeTransferToken,
		tx.Amount,
		tx.FromAddress, tx.ToAddress, m.EthConfig.TokenAddress,
		nil,
		gasPrice,
		m.TxConfig.TransferGas,
		chainID.String(),
	)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create a transfer transaction")
	}

	taxNonce := nonce + 1
	taxTx, _, err := m.createTransfer(
		ctx,
		database.TransferTypeTax,
		tax,
		tx.FromAddress, m.EthConfig.BankAddress, m.EthConfig.TokenAddress,
		&taxNonce,
		gasPrice,
		m.TxConfig.TaxGas,
		chainID.String(),
	)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create a tax transaction")
	}

	// save transfers to the db
	newDbTransfers := []*database.NewTransfer{
		database.NewTransferFromPublic(transferTx, tx.FromAddress, taxTx.Amount), // transfer first
		database.NewTransferFromPublic(taxTx, tx.FromAddress, ""),
	}
	dbTransfers, err := m.DB.CreateTransfers(ctx, newDbTransfers)
	if err != nil {
		return nil, err
	}

	// add the ids to confirm transfers later
	transferTx.ID = dbTransfers[0].ID // transfer first as above
	taxTx.ID = dbTransfers[1].ID
	return []*models.UnsignedTransfer{transferTx, taxTx}, nil
}

func (m *Manager) ConfirmTransfer(ctx context.Context, transfers *models.SignedTransfer) (
	*models.QueuedTransfer, error,
) {
	log.AddFields(ctx, "transfers", transfers)

	fromAddr := common.HexToAddress(transfers.FromAddress)
	accNonce, err := m.EthClient.PendingNonceAt(ctx, fromAddr)
	if err != nil {
		for _, t := range transfers.Transactions { // fail transfers
			_ = m.DB.WriteTransferError(ctx, t.ID, err.Error())
		}
		return nil, errors.Wrap(err, "failed to retrieve account nonce")
	}

	if err := transfers.Validate(func(nonce uint64) error {
		if accNonce > nonce {
			return errors.New("account nonce is already greater than nonce for the current transfer, you need to create a new transfer")
		}
		if m.DB.IsNoncePending(ctx, nonce, transfers.FromAddress) {
			return errors.New("nonce is already used by queued or pending transfer, you need to create a new transfer")
		}
		return nil
	}); err != nil {
		for _, t := range transfers.Transactions { // fail transfers
			_ = m.DB.WriteTransferError(ctx, t.ID, err.Error())
		}
		return nil, err
	}

	// update tx hashes and statuses for transfers and calculate gas fee for transfers in wei
	var dbTransfers []*database.Transfer
	gasFee := new(big.Int)
	for _, t := range transfers.Transactions {
		dbt, err := m.DB.GetAndConfirmTransferByID(ctx, t.ID, transfers.FromAddress)
		if err != nil {
			return nil, errors.Wrap(err, "failed to get a db transfer")
		}

		// do not allow double transfers
		if dbt.IsProcessing() {
			return nil, errors.New("you have already confirmed this transfer")
		}

		dbt.RawTx = &t.RawTx
		dbt.Status = database.TransferStatusQueued
		dbTransfers = append(dbTransfers, dbt)

		gasPrice, _ := new(big.Int).SetString(dbt.GasPrice, 10)
		currentGasFee := eth.CalcGasCost(dbt.GasLimit, gasPrice)
		gasFee.Add(gasFee, currentGasFee)
	}

	// request the required gas fee from the service wallet
	// and wait for scrapper to complete the transfer

	// create and send an eth transaction
	dbEthTransfer, err := m.topUpEthBalance(ctx, transfers.FromAddress, gasFee)
	if err != nil {
		msg := "failed to send an eth transaction to top up the wallet balance"
		for _, t := range transfers.Transactions { // fail transfers
			_ = m.DB.WriteTransferError(ctx, t.ID, msg)
		}
		return nil, errors.Wrap(err, msg)
	}

	// update statuses and wait for eth top up completion
	for _, dbt := range dbTransfers {
		dbt.EthTransferID = &dbEthTransfer.ID
	}
	// update db transfers
	dbTransfers, err = m.DB.UpdateTransfers(ctx, dbTransfers)
	if err != nil {
		return nil, err
	}

	// render result
	var result []*models.QueuedTransaction
	for _, dbt := range dbTransfers {
		result = append(result, &models.QueuedTransaction{
			ID:     dbt.ID,
			Status: dbt.Status,
		})
	}
	return &models.QueuedTransfer{Transactions: result}, nil
}

func (m *Manager) CompleteTransfers(ctx context.Context, ethTransferID string) error {
	// get transfers and calculate gas fee for transfers in wei
	dbTransfers, err := m.DB.GetQueuedTransfersFor(ctx, ethTransferID)
	if err != nil {
		return err
	}
	if dbTransfers == nil {
		return errors.New("empty transfers array received from the db")
	}

	if err := m.DB.StartEthTransferProcessing(ctx, ethTransferID); err != nil {
		log.AddFields(ctx, "start eth transfer processing error", err.Error())
	}

	var fromAddress string
	gasFee := new(big.Int)
	for _, dbt := range dbTransfers {
		fromAddress = dbt.FromAddress

		gasPrice, _ := new(big.Int).SetString(dbt.GasPrice, 10)
		currentGasFee := eth.CalcGasCost(dbt.GasLimit, gasPrice)
		gasFee.Add(gasFee, currentGasFee)
	}

	// get account balance and compare it to the required gas fee
	balance, err := m.EthClient.BalanceAt(ctx, common.HexToAddress(fromAddress), nil)
	if err != nil {
		err = errors.Wrap(err, "failed to retrieve account balance")
		// save error message to the transfers
		for _, dbt := range dbTransfers {
			_ = m.DB.WriteTransferError(ctx, dbt.ID, err.Error())
		}
		return err
	}

	if balance.Cmp(gasFee) < 0 { // there are NO enough eth to complete the transfers on the wallet
		err = errors.Errorf("there are no enough ETH on the wallet %s to pay the gas fee", fromAddress)
		// save error message to the transfers
		for _, dbt := range dbTransfers {
			_ = m.DB.WriteTransferError(ctx, dbt.ID, err.Error())
		}
		return err
	}

	// send raw transactions and update statuses
	for _, dbt := range dbTransfers {
		var txHash common.Hash
		if err = m.RpcClient.CallContext(
			ctx, &txHash, "eth_sendRawTransaction", *dbt.RawTx,
		); err != nil {
			err = errors.Wrap(err, "failed to send a raw transaction")
			_ = m.DB.WriteTransferError(ctx, dbt.ID, err.Error()) // save error message to the transfer
			return err
		}

		dbt.Status = database.TransferStatusPending
		hexTxHash := txHash.String()
		dbt.TxHash = &hexTxHash
	}

	// complete eth top up
	if err = m.DB.CompleteTransfer(ctx, ethTransferID, ""); err != nil {
		log.AddFields(ctx, "complete eth transfer error", err.Error())
	}

	// update db transfers
	_, err = m.DB.UpdateTransfers(ctx, dbTransfers)
	return err
}

func (m *Manager) TransactionHistory(ctx context.Context, filter *models.TransactionHistoryFilter) (
	*models.TransactionHistory, error,
) {
	log.AddFields(ctx, "filter", filter)

	if err := filter.Validate(); err != nil {
		return nil, err
	}

	transactions, total, err := m.DB.TransactionHistory(ctx, &database.TransactionHistoryFilter{
		Address:     filter.Address,
		After:       filter.After,
		BankAddress: m.EthConfig.BankAddress,
		Skip:        filter.Skip,
		Limit:       filter.Limit,
	})
	if err != nil {
		return nil, err
	}

	decimals, err := m.getTokenDecimals(ctx)
	if err != nil {
		return nil, err
	}

	var result []*models.TransactionHistoryItem
	for _, t := range transactions {
		result = append(result, t.ToPublic(decimals))
	}

	return &models.TransactionHistory{
		Transactions: result,
		Meta:         &models.ListMeta{Total: total},
	}, nil
}

func (m *Manager) GetTransaction(ctx context.Context, filter *models.TransactionFilter) (
	*models.TransactionHistoryItem, error,
) {
	log.AddFields(ctx, "filter", filter)

	if err := filter.Validate(); err != nil {
		return nil, err
	}

	tx, err := m.DB.GetTransactionOrTransfer(ctx, &database.TransactionFilter{
		Address: filter.Address,
		Hash:    filter.Hash,
	})
	if err != nil {
		return nil, err
	}

	decimals, err := m.getTokenDecimals(ctx)
	if err != nil {
		return nil, err
	}

	result := tx.ToPublic(decimals)

	// get final status and eth fee (for completed txes)
	if result.IsCompleted() {
		rcpt, err := m.EthClient.TransactionReceipt(ctx, common.HexToHash(result.TxHash))
		if err != nil {
			return nil, errors.Wrap(err, "failed to retrieve tx receipt")
		}
		result.Status = models.TxStatusConfirmed
		if rcpt.Status < 1 {
			result.Status = models.TxStatusFailed
		}

		// calculate fee
		transaction, _, err := m.EthClient.TransactionByHash(ctx, common.HexToHash(result.TxHash))
		if err != nil {
			return nil, errors.Wrap(err, "failed to fetch tx by hash")
		}

		gasFee := eth.CalcGasCost(rcpt.GasUsed, transaction.GasPrice())
		ethGasFee := eth.ToETH(gasFee, 18)
		fethGasFee, _ := ethGasFee.Float64()
		result.EthFee = fethGasFee
	}

	// calculate sdr fee
	transfer, err := m.DB.GetTransferByIDOrHash(ctx, filter.Hash, result.TxHash)
	if err != nil {
		log.AddFields(ctx, "note", err.Error())
		return result, nil
	}

	sdrFee := eth.ToETH(transfer.TaxAmount, decimals)
	fsdrFee, _ := sdrFee.Float64()
	result.SdrFee = fsdrFee
	//result.EthFee *= 2 // because we have two transactions per a transfer

	return result, nil
}

func (m *Manager) estimateTax(ctx context.Context, amount float64) (tax float64, gasPrice *big.Int, err error) {
	// calculate gas price in wei and convert to ETH
	gasPrice, err = m.EthClient.SuggestGasPrice(ctx)
	if err != nil {
		err = errors.Wrap(err, "failed to suggest gas price")
		return
	}
	gasFee := eth.CalcGasCost(m.estimateTotalGasLimit(), gasPrice)
	gasFeeETH := eth.ToETH(gasFee, 18) // 18 defined by etherium network

	// get sdr rate
	sdrRate, err := m.SdrSidecar.EthToSdr(ctx)
	if err != nil {
		err = errors.Wrap(err, "failed to get ETH to SDR rate")
		return
	}

	// calc gasFee in sdr
	gasFeeSDR := gasFeeETH.Mul(decimal.NewFromFloat(sdrRate))

	// make tax a little bit higher than gas fee
	addend := gasFeeSDR.Mul(decimal.NewFromFloat(m.TxConfig.Surcharge))
	gasFeeSDR = gasFeeSDR.Add(addend)
	tax, _ = gasFeeSDR.Float64()
	return
}

func (m *Manager) estimateTotalGasLimit() uint64 {
	// 2 token transfers + eth top up
	return m.TxConfig.TransferGas + m.TxConfig.TaxGas + ethTxGasLimit
}

func (m *Manager) createTransfer(
	ctx context.Context,
	transferType string,
	amount float64,
	fromAddress, toAddress, tokenAddress string,
	suggestNonce *uint64,
	gasPrice *big.Int,
	gasLimit uint64,
	chainID string,
) (
	*models.UnsignedTransfer, uint64, error,
) {
	// get token amount in the smallest units
	decimals, err := m.getTokenDecimals(ctx)
	if err != nil {
		return nil, 0, err
	}
	wei := eth.ToWei(amount, decimals)

	fromAddr := common.HexToAddress(fromAddress)
	toAddr := common.HexToAddress(toAddress)
	tokenAddr := common.HexToAddress(tokenAddress)

	// get nonce
	var nonce uint64
	if suggestNonce == nil {
		nonce, err = m.EthClient.PendingNonceAt(ctx, fromAddr)
		if err != nil {
			return nil, 0, errors.Wrap(err, "failed to retrieve account nonce")
		}
	} else {
		nonce = *suggestNonce
	}

	// collect input
	paddedAddress := common.LeftPadBytes(toAddr.Bytes(), 32)
	paddedAmount := common.LeftPadBytes(wei.Bytes(), 32)

	var input []byte
	input = append(input, transferMethodID...)
	input = append(input, paddedAddress...)
	input = append(input, paddedAmount...)

	// create unsigned transaction
	tx := types.NewTransaction(nonce, tokenAddr, big.NewInt(0), gasLimit, gasPrice, input)
	return &models.UnsignedTransfer{
		TransferType: transferType,
		Amount:       wei.String(),
		ToAddress:    toAddress,
		TxData: &models.UnsignedTransaction{
			Nonce:    hexutil.EncodeUint64(tx.Nonce()),
			GasPrice: hexutil.EncodeBig(tx.GasPrice()),
			GasLimit: hexutil.EncodeUint64(tx.Gas()),
			To:       tx.To().String(),
			Value:    hexutil.EncodeBig(tx.Value()),
			ChainID:  chainID,
			Data:     hexutil.Encode(tx.Data()),
		},
	}, nonce, nil
}

func (m *Manager) topUpEthBalance(ctx context.Context, toAddress string, amount *big.Int) (*database.Transfer, error) {
	privateKey, err := crypto.HexToECDSA(m.EthConfig.BankPrivateKey)
	if err != nil {
		return nil, errors.Wrap(err, "failed to convert private key to ecdsa")
	}

	publicKey := privateKey.Public()
	publicKeyECDSA, ok := publicKey.(*ecdsa.PublicKey)
	if !ok {
		return nil, errors.Wrap(err, "failed to case public key to ecdsa")
	}
	fromAddress := crypto.PubkeyToAddress(*publicKeyECDSA)

	nonce, err := m.EthClient.PendingNonceAt(ctx, fromAddress)
	if err != nil {
		return nil, errors.Wrap(err, "failed to retrieve service wallet nonce")
	}

	// calc gas fee
	gasLimit := uint64(ethTxGasLimit) // defined by Etherium network
	gasPrice, err := m.EthClient.SuggestGasPrice(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to estimate gas needed to top up the account balance")
	}
	gasFee := eth.CalcGasCost(gasLimit, gasPrice)
	totalExpenses := new(big.Int).Add(amount, gasFee)

	// check service wallet balance
	balance, err := m.EthClient.BalanceAt(ctx, fromAddress, nil)
	if err != nil {
		return nil, errors.Wrap(err, "failed to retrieve service wallet balance")
	}
	if balance.Cmp(totalExpenses) < 0 {
		return nil, errors.New("service wallet balance is not enough to complete the ETH transaction, contact the support")
	}

	// create transaction
	toAddr := common.HexToAddress(toAddress)
	var data []byte
	tx := types.NewTransaction(nonce, toAddr, amount, gasLimit, gasPrice, data)

	chainID, err := m.EthClient.NetworkID(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get chain id")
	}

	signedTx, err := types.SignTx(tx, types.NewEIP155Signer(chainID), privateKey)
	if err != nil {
		return nil, errors.Wrap(err, "failed to sign an ETH transaction")
	}

	if err = m.EthClient.SendTransaction(ctx, signedTx); err != nil {
		return nil, errors.Wrap(err, "failed to send an ETH transaction")
	}

	// save transfer to the db
	return m.DB.CreateEthTransfer(
		ctx,
		&database.NewTransfer{
			FromAddress:  fromAddress.String(),
			Nonce:        nonce,
			GasPrice:     gasPrice.String(),
			GasLimit:     gasLimit,
			TransferType: database.TransferTypeEthTopUp,
			Amount:       amount.String(),
			ToAddress:    toAddress,
		},
		signedTx.Hash().String(),
	)
}

func (m *Manager) getTokenDecimals(ctx context.Context) (uint8, error) {
	if m.tokenDecimals != 0 {
		return m.tokenDecimals, nil
	}
	decimals, err := m.SdrToken.Decimals(&bind.CallOpts{Context: ctx})
	if err != nil {
		return 0, errors.Wrap(err, "failed to get token's decimals")
	}
	m.tokenDecimals = decimals
	return m.tokenDecimals, nil
}
