package main

import (
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	"github.com/go-chi/jwtauth"

	"backend/app/auth"
	"backend/app/config"
	"backend/app/notifier"
	"backend/app/scrapper"
	"backend/app/scrapper/scrappers"
	"backend/app/scrapper/token"
	"backend/app/sdrsidecar"
	"backend/app/server"
	"backend/app/storage/database"
	"backend/app/transaction"
	"backend/app/wallet"
	"backend/pkg/eth"
	"backend/pkg/log"
	"backend/pkg/web"
	webware "backend/pkg/web/middleware"
)

const (
	maxRequestsAllowed    = 10000
	serverShutdownTimeout = 30 * time.Second
	wsReconnectInterval   = 30 * time.Minute
)

func main() {
	cfg, err := config.Parse()
	if err != nil {
		panic(err)
	}

	zlog := log.ConfigureLogger(cfg.Logging)
	defer func() {
		_ = zlog.Sync() // flush the logger
	}()

	// connect to the database
	db, err := database.Connect(cfg.Database)
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		_ = db.Close()
	}()

	// connect to the node
	ethClient, rpcClient, err := eth.Dial(cfg.Ethereum.NodeUrl)
	if err != nil {
		log.Fatal("failed connection to node: ", err)
	}

	contractABI, err := abi.JSON(strings.NewReader(token.TokenABI))
	if err != nil {
		log.Fatal("failed to parse a token contract: ", err)
	}
	sdrToken, err := token.NewToken(common.HexToAddress(cfg.Ethereum.TokenAddress), ethClient)
	if err != nil {
		log.Fatal("failed to instantiate a token contract: ", err)
	}

	sdrSidecarSvc := &sdrsidecar.Manager{
		Config: cfg.SdrBackend,
		HttpClient: &http.Client{
			Timeout: time.Second * 30,
		},
	}

	transactionSvc := &transaction.Manager{
		TxConfig:   cfg.Transaction,
		EthConfig:  cfg.Ethereum,
		DB:         db,
		EthClient:  ethClient,
		RpcClient:  rpcClient,
		SdrToken:   sdrToken,
		SdrSidecar: sdrSidecarSvc,
	}

	notifierSvc := notifier.NewManager()

	// start scrappers
	scrapperSvc := scrapper.NewManager(cfg.Ethereum, db, transactionSvc, notifierSvc)
	eventScraper := scrappers.NewEventsScraper(sdrToken, ethClient, scrapperSvc, cfg.Ethereum)
	if err = eventScraper.Start(); err != nil {
		log.Fatal("failed to start event scrapper: ", err)
	}

	txScraper := scrappers.NewTxScraper(ethClient, scrapperSvc, cfg.Ethereum, contractABI)
	if err = txScraper.Start(); err != nil {
		log.Fatal("failed to start event scrapper: ", err)
	}

	connClosed := make(chan struct{})
	go scrappersListen(eventScraper, txScraper, cfg.Ethereum, connClosed)

	router := newRouter()
	authSvc := &auth.Manager{
		JWTAuth: jwtauth.New("HS256", []byte(cfg.Secrets.Token), nil),
	}
	walletSvc := &wallet.Manager{
		DB:        db,
		Secrets:   cfg.Secrets,
		Auth:      authSvc,
		EthClient: ethClient,
		SdrToken:  sdrToken,
		Scrapper:  scrapperSvc,
	}
	rest := server.Rest{
		Router:      router,
		Wallet:      walletSvc,
		Notifier:    notifierSvc,
		Auth:        authSvc,
		Transaction: transactionSvc,
	}
	rest.Route() // handle http requests

	// start notifier an http server and remember to shut it down
	srv := &http.Server{
		Addr:    cfg.RestAddr,
		Handler: router,
	}
	go notifierSvc.Start()
	go web.Start(srv)
	defer web.Shutdown(srv, serverShutdownTimeout)

	// wait for the program exit
	exit := make(chan os.Signal, 1)
	signal.Notify(exit, os.Interrupt, syscall.SIGTERM)
	select {
	case <-exit:
	case <-connClosed:
	}
}

func newRouter() chi.Router {
	router := chi.NewRouter()

	// add middleware
	router.Use(
		middleware.Throttle(maxRequestsAllowed),
		middleware.RealIP,
		webware.ZapLogger,
		webware.Recoverer,
	)

	return router
}

func scrappersListen(
	eventScrapper *scrappers.EventsScraper,
	txScrapper *scrappers.TxScraper,
	cfg config.Ethereum,
	connClosed chan<- struct{},
) {
	for {
		wsEthClient, _, err := eth.Dial(cfg.WsNodeUrl)
		if err != nil {
			connClosed <- struct{}{}
			log.Error("failed to connect to node by ws: ", err)
		}

		wsSdrToken, err := token.NewToken(common.HexToAddress(cfg.TokenAddress), wsEthClient)
		if err != nil {
			connClosed <- struct{}{}
			log.Error("failed to instantiate a token contract by ws: ", err)
		}

		loopNext, stopEvents, stopTxes := make(chan struct{}, 1), make(chan struct{}, 1), make(chan struct{}, 1)
		go func() {
			if err := eventScrapper.Listen(wsSdrToken, stopEvents); err != nil {
				log.Error(err)
			}
			// connection is closed, reconnect
			stopTxes <- struct{}{}
			loopNext <- struct{}{}
		}()

		go func() {
			if err := txScrapper.Listen(wsEthClient, stopTxes); err != nil {
				log.Error(err)
			}
			// connection is closed, reconnect
			stopEvents <- struct{}{}
			loopNext <- struct{}{}
		}()

		select {
		case <-loopNext:
			log.Info("automatically reconnect ws")
		case <-time.After(wsReconnectInterval):
			log.Info("manually reconnect ws by timer")
		}
		wsEthClient.Close() // in case of manual reconnect
	}
}
