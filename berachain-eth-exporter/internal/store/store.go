package store

import (
	"database/sql"
	"fmt"
	"sync"

	"eth-exporter/internal/types"
)

type Store struct {
	db   *sql.DB
	mu   sync.RWMutex
	conf types.DBConfig
}

func NewStore(conf types.DBConfig) (*Store, error) {
	db, err := sql.Open(conf.Driver, conf.DSN)
	if err != nil {
		return nil, err
	}

	store := &Store{
		db:   db,
		conf: conf,
	}

	if err := store.initialize(); err != nil {
		return nil, err
	}

	return store, nil
}

func (s *Store) initialize() error {
	var schema string
	if s.conf.Driver == "mysql" {
		schema = `
			CREATE TABLE IF NOT EXISTS blocks (
				number BIGINT PRIMARY KEY,
				miner VARCHAR(42) NOT NULL,
				validator_name VARCHAR(100),
				client VARCHAR(50),
				version VARCHAR(50),
				extra_data_hex TEXT,
				tx_count INT DEFAULT 0,
				is_empty BOOLEAN DEFAULT 0,
				timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
				INDEX idx_miner (miner),
				INDEX idx_client (client),
				INDEX idx_validator_name (validator_name),
				INDEX idx_timestamp (timestamp),
				INDEX idx_is_empty (is_empty)
			)`
	} else {
		schema = `
			CREATE TABLE IF NOT EXISTS blocks (
				number INTEGER PRIMARY KEY,
				miner TEXT NOT NULL,
				validator_name TEXT,
				client TEXT,
				version TEXT,
				extra_data_hex TEXT,
				tx_count INTEGER DEFAULT 0,
				is_empty BOOLEAN DEFAULT 0,
				timestamp DATETIME DEFAULT CURRENT_TIMESTAMP
			);
			CREATE INDEX IF NOT EXISTS idx_miner ON blocks(miner);
			CREATE INDEX IF NOT EXISTS idx_client ON blocks(client);
			CREATE INDEX IF NOT EXISTS idx_validator_name ON blocks(validator_name);
			CREATE INDEX IF NOT EXISTS idx_timestamp ON blocks(timestamp);
			CREATE INDEX IF NOT EXISTS idx_is_empty ON blocks(is_empty);`
	}

	if _, err := s.db.Exec(schema); err != nil {
		return fmt.Errorf("failed to initialize database: %v", err)
	}

	return nil
}

func (s *Store) GetLastProcessedBlock() (uint64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var lastBlock uint64
	err := s.db.QueryRow("SELECT COALESCE(MAX(number), 0) FROM blocks").Scan(&lastBlock)
	if err != nil {
		return 0, err
	}

	return lastBlock, nil
}

func (s *Store) StoreBlock(info types.BlockInfo) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	query := `
		INSERT INTO blocks 
		(number, miner, validator_name, client, version, extra_data_hex, tx_count, is_empty, timestamp)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`

	if s.conf.Driver == "mysql" {
		query += ` ON DUPLICATE KEY UPDATE
			miner=VALUES(miner),
			validator_name=VALUES(validator_name),
			client=VALUES(client),
			version=VALUES(version),
			extra_data_hex=VALUES(extra_data_hex),
			tx_count=VALUES(tx_count),
			is_empty=VALUES(is_empty),
			timestamp=VALUES(timestamp)`
	} else {
		query = `
			INSERT OR REPLACE INTO blocks 
			(number, miner, validator_name, client, version, extra_data_hex, tx_count, is_empty, timestamp)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`
	}

	_, err := s.db.Exec(query,
		info.Number,
		info.Miner,
		info.ValidatorName,
		info.Client,
		info.Version,
		info.ExtraDataHex,
		info.TxCount,
		info.IsEmpty,
		info.Timestamp,
	)

	return err
}

func (s *Store) GetClientStats() (map[string]map[string]map[string]int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	clientStats := make(map[string]map[string]map[string]int)

	var query string
	if s.conf.Driver == "mysql" {
		query = `
			SELECT client, version, validator_name, COUNT(*) as count
			FROM blocks
			WHERE timestamp > DATE_SUB(NOW(), INTERVAL 1 DAY)
			GROUP BY client, version, validator_name`
	} else {
		query = `
			SELECT client, version, validator_name, COUNT(*) as count
			FROM blocks
			WHERE timestamp > datetime('now', '-1 day')
			GROUP BY client, version, validator_name`
	}

	rows, err := s.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var client, version, validatorName string
		var count int
		if err := rows.Scan(&client, &version, &validatorName, &count); err != nil {
			return nil, err
		}

		if _, exists := clientStats[client]; !exists {
			clientStats[client] = make(map[string]map[string]int)
		}
		if _, exists := clientStats[client][version]; !exists {
			clientStats[client][version] = make(map[string]int)
		}
		clientStats[client][version][validatorName] = count
	}

	return clientStats, nil
}

func (s *Store) GetMinerStats() (map[string]map[string]map[string]int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	minerStats := make(map[string]map[string]map[string]int)

	var query string
	if s.conf.Driver == "mysql" {
		query = `
			SELECT miner, client, validator_name, COUNT(*) as count
			FROM blocks
			WHERE timestamp > DATE_SUB(NOW(), INTERVAL 1 DAY)
			GROUP BY miner, client, validator_name`
	} else {
		query = `
			SELECT miner, client, validator_name, COUNT(*) as count
			FROM blocks
			WHERE timestamp > datetime('now', '-1 day')
			GROUP BY miner, client, validator_name`
	}

	rows, err := s.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var miner, client, validatorName string
		var count int
		if err := rows.Scan(&miner, &client, &validatorName, &count); err != nil {
			return nil, err
		}

		if _, exists := minerStats[miner]; !exists {
			minerStats[miner] = make(map[string]map[string]int)
		}
		if _, exists := minerStats[miner][client]; !exists {
			minerStats[miner][client] = make(map[string]int)
		}
		minerStats[miner][client][validatorName] = count
	}

	return minerStats, nil
}

func (s *Store) GetEmptyBlockStats() (map[string]int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := make(map[string]int)

	var query string
	if s.conf.Driver == "mysql" {
		query = `
			SELECT validator_name, COUNT(*) as count
			FROM blocks
			WHERE is_empty = 1 AND timestamp > DATE_SUB(NOW(), INTERVAL 1 DAY)
			GROUP BY validator_name`
	} else {
		query = `
			SELECT validator_name, COUNT(*) as count
			FROM blocks
			WHERE is_empty = 1 AND timestamp > datetime('now', '-1 day')
			GROUP BY validator_name`
	}

	rows, err := s.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var validatorName string
		var count int
		if err := rows.Scan(&validatorName, &count); err != nil {
			return nil, err
		}
		stats[validatorName] = count
	}

	return stats, nil
}
