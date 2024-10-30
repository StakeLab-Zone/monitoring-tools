package types

import (
	"sync"
	"time"
)

type BlockInfo struct {
	Number        uint64
	Miner         string
	ValidatorName string
	Client        string
	Version       string
	ExtraDataHex  string
	TxCount       int
	IsEmpty       bool
	Timestamp     time.Time
}

type DBConfig struct {
	Driver string
	DSN    string
}

type Config struct {
	StartBlock   uint64
	CurrentBlock uint64
	Mu           sync.RWMutex
}

var (
	ValidatorNames = make(map[string]string)
	ClientEmojis   = map[string]string{
		"Geth":       "🟢",
		"RETH":       "🔵",
		"Nethermind": "🟣",
		"Erigon":     "🟡",
		"Besu":       "🟤",
		"EthereumJS": "⚪",
		"Unknown":    "❓",
	}
)
