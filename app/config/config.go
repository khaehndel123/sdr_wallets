package config

import (
	"flag"

	"github.com/pkg/errors"
	"github.com/spf13/viper"

	"backend/app/storage/database"
	"backend/pkg/log"
)

const (
	defaultConfigPath = "./configs/config.yaml"

	defaultRestAddr        = ":8000"
	defaultMigrationsTable = "wallet_schema_migrations"
)

type Ethereum struct {
	NodeUrl        string `mapstructure:"nodeUrl"`
	WsNodeUrl      string `mapstructure:"wsNodeUrl"`
	TokenAddress   string `mapstructure:"tokenAddress"`
	BankAddress    string `mapstructure:"bankAddress"`
	BankPrivateKey string `mapstructure:"bankPrivateKey"`
	PacketSize     uint64 `mapstructure:"packetSize"`
}

func (e *Ethereum) Validate() error {
	if e.NodeUrl == "" {
		return errors.New("you must provide eth node url in a config")
	}

	if e.WsNodeUrl == "" {
		return errors.New("you must provide eth WS node url in a config")
	}

	if e.TokenAddress == "" {
		return errors.New("you must provide eth token address in a config")
	}

	if e.BankAddress == "" {
		return errors.New("you must provide eth bank address in a config")
	}

	if e.BankPrivateKey == "" {
		return errors.New("you must provide eth bank private key in a config")
	}

	if e.PacketSize == 0 {
		return errors.New("you must provide eth packet size in a config")
	}

	return nil
}

type Secrets struct {
	API   string `mapstructure:"api"`
	Token string `mapstructure:"token"`
}

func (s *Secrets) Validate() error {
	if s.API == "" || s.Token == "" {
		return errors.New("you must provide secrets in a config")
	}
	return nil
}

type Transaction struct {
	Surcharge   float64 `mapstructure:"surcharge"`
	TransferGas uint64  `mapstructure:"transferGas"`
	TaxGas      uint64  `mapstructure:"taxGas"`
}

func (c *Transaction) Validate() error {
	if c.Surcharge == 0 {
		return errors.New("you must provide transaction surcharge in a config")
	}

	if c.TransferGas == 0 {
		return errors.New("you must provide transfer gas limit in a config")
	}

	if c.TaxGas == 0 {
		return errors.New("you must provide tax gas limit in a config")
	}

	return nil
}

type SdrBackend struct {
	BasePath string `mapstructure:"basePath"`
	ApiKey   string `mapstructure:"apiKey"`
}

func (s *SdrBackend) Validate() error {
	if s.BasePath == "" {
		return errors.New("you must provide base path for SDR backend")
	}

	if s.ApiKey == "" {
		return errors.New("you must provide api key for SDR backend")
	}

	return nil
}

type Config struct {
	RestAddr    string          `mapstructure:"restAddr"`
	Ethereum    Ethereum        `mapstructure:"ethereum"`
	Secrets     Secrets         `mapstructure:"secrets"`
	Database    database.Config `mapstructure:"database"`
	Logging     log.Config      `mapstructure:"log"`
	Transaction Transaction     `mapstructure:"transaction"`
	SdrBackend  SdrBackend      `mapstructure:"sdrBackend"`
}

func Parse() (*Config, error) {
	configPath := flag.String("config", defaultConfigPath, "configuration file path")
	flag.Parse()

	// set reasonable defaults
	viper.SetDefault("restAddr", defaultRestAddr)
	viper.SetDefault("database.migrationsTable", defaultMigrationsTable)

	// read a config file
	viper.SetConfigFile(*configPath)
	if err := viper.ReadInConfig(); err != nil {
		return nil, errors.Wrap(err, "failed to read a file")
	}

	// unmarshal to a config struct
	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal a config")
	}

	// ensure etherium config is valid
	if err := cfg.Ethereum.Validate(); err != nil {
		return nil, err
	}

	// ensure secrets are provided
	if err := cfg.Secrets.Validate(); err != nil {
		return nil, err
	}

	// ensure tax is provided
	if err := cfg.Transaction.Validate(); err != nil {
		return nil, err
	}

	return &cfg, nil
}
