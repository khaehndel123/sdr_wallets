package wallet

import (
	"context"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/pkg/errors"

	"backend/app/auth"
	"backend/app/config"
	"backend/app/models"
	"backend/app/scrapper"
	"backend/app/scrapper/token"
	"backend/app/storage/database"
	"backend/pkg/eth"
	"backend/pkg/log"
)

type Manager struct {
	DB        database.Database
	Secrets   config.Secrets
	Auth      auth.Service
	EthClient *ethclient.Client
	SdrToken  *token.Token
	Scrapper  scrapper.Service

	tokenDecimals uint8 // cached version
}

func (m *Manager) CreateWallet(ctx context.Context, wallet *models.NewWallet) (*models.AuthorizedWallet, error) {
	log.AddFields(ctx, "wallet", wallet)

	if err := wallet.Validate(m.Secrets.API); err != nil {
		return nil, err
	}

	dbWallet, err := m.DB.CreateWallet(ctx, database.NewWalletFromPublic(wallet))
	if err != nil {
		return nil, err
	}

	wlt := dbWallet.ToPublic()
	m.Scrapper.RegisterWallet(ctx, wlt.Address)
	accessToken, err := m.Auth.IssueAccessToken(ctx, wlt)
	if err != nil {
		return nil, err
	}

	return &models.AuthorizedWallet{
		Wallet:      wlt,
		AccessToken: accessToken,
	}, nil
}

func (m *Manager) GetWallet(ctx context.Context, filter *models.GetWallet) (*models.AuthorizedWallet, error) {
	log.AddFields(ctx, "filter", filter)

	if err := filter.Validate(m.Secrets.API); err != nil {
		return nil, err
	}

	dbWallet, err := m.DB.GetWallet(ctx, filter.Address)
	if err != nil {
		return nil, err
	}

	wlt := dbWallet.ToPublic()
	accessToken, err := m.Auth.IssueAccessToken(ctx, wlt)
	if err != nil {
		return nil, err
	}

	return &models.AuthorizedWallet{
		Wallet:      wlt,
		AccessToken: accessToken,
	}, nil
}

func (m *Manager) GetBalance(ctx context.Context, address string) (*models.Balance, error) {
	sdrBalance, err := m.SdrToken.BalanceOf(&bind.CallOpts{Context: ctx}, common.HexToAddress(address))
	if err != nil {
		return nil, errors.Wrap(err, "failed to check token balance")
	}
	decimals, err := m.getTokenDecimals(ctx)
	if err != nil {
		return nil, err
	}

	dsdr := eth.ToETH(sdrBalance, decimals)
	fsdr, _ := dsdr.Float64()

	return &models.Balance{SDR: fsdr}, nil
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
