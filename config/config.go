package config

import (
	"github.com/filecoin-project/venus/pkg/types"
	"time"
)

type Config struct {
	API            APIConfig            `toml:"api"`
	Log            LogConfig            `toml:"log"`
	DB             DbConfig             `toml:"db"`
	JWT            JWTConfig            `toml:"jwt"`
	Node           NodeConfig           `toml:"node"`
	MessageService MessageServiceConfig `toml:"messageService"`
	DataTransfer   TransferConfig       `toml:"data-transfer"`
	MarketFee      MarketFeeConfig
}

type MarketFeeConfig struct {
	MaxPublishDealsFee              types.FIL
	MaxMarketBalanceAddFee          types.FIL
	MaxProviderCollateralMultiplier uint64
}

type TransferConfig struct {
	MetaDs string `toml:"path"`
	Path   string `toml:"path"`
}

type NodeConfig struct {
	Url   string `toml:"url"`
	Token string `toml:"token"`
}

type LogConfig struct {
	Path  string `toml:"path"`
	Level string `toml:"level"`
}

type APIConfig struct {
	Address string
}

type DbConfig struct {
	Type   string       `toml:"type"`
	MySql  MySqlConfig  `toml:"mysql"`
	Sqlite SqliteConfig `toml:"sqlite"`
}

type SqliteConfig struct {
	Path  string `toml:"path"`
	Debug bool   `toml:"debug"`
}

type MySqlConfig struct {
	ConnectionString string        `toml:"connectionString"`
	MaxOpenConn      int           `toml:"maxOpenConn"`
	MaxIdleConn      int           `toml:"maxIdleConn"`
	ConnMaxLifeTime  time.Duration `toml:"connMaxLifeTime"`
	Debug            bool          `toml:"debug"`
}

type JWTConfig struct {
	Url string `toml:"url"`
}

type MessageServiceConfig struct {
	Url    string `toml:"url"`
	Token  string `toml:"token"`
	Wallet string `toml:"wallet"`
}

func DefaultConfig() *Config {
	return &Config{
		DB: DbConfig{
			Type: "sqlite",
			MySql: MySqlConfig{
				ConnectionString: "",
				MaxOpenConn:      10,
				MaxIdleConn:      10,
				ConnMaxLifeTime:  time.Second * 60,
				Debug:            false,
			},
			Sqlite: SqliteConfig{Path: "./message.db"},
		},
		JWT: JWTConfig{
			Url: "http://127.0.0.1:8989",
		},
		Log: LogConfig{
			Path:  "messager.log",
			Level: "info",
		},
		API: APIConfig{
			Address: "0.0.0.0:39812",
		},
		Node: NodeConfig{
			Url:   "",
			Token: "",
		},
		MessageService: MessageServiceConfig{},
	}
}
