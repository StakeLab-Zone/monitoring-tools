package main

import (
	"context"
	"encoding/hex"
	"flag"
	"fmt"
	"log"
	"math/big"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/ethclient"
	ethRpc "github.com/ethereum/go-ethereum/rpc"
	_ "github.com/go-sql-driver/mysql"
	_ "github.com/mattn/go-sqlite3"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"gopkg.in/yaml.v3"

	"eth-exporter/internal/detector"
	"eth-exporter/internal/metrics"
	"eth-exporter/internal/store"
	"eth-exporter/internal/types"
)

type Exporter struct {
	client    *ethclient.Client
	rpcClient *ethRpc.Client
	store     *store.Store
	metrics   *metrics.Metrics
	config    *types.Config
	batchSize uint64
	detector  *detector.ClientDetector
}

func normalizeAddress(address string) string {
	addr := strings.ToLower(address)
	if !strings.HasPrefix(addr, "0x") {
		addr = "0x" + addr
	}
	return addr
}

func (e *Exporter) updateMetrics() error {
	clientStats, err := e.store.GetClientStats()
	if err != nil {
		return fmt.Errorf("failed to get client stats: %v", err)
	}

	minerStats, err := e.store.GetMinerStats()
	if err != nil {
		return fmt.Errorf("failed to get miner stats: %v", err)
	}

	emptyBlockStats, err := e.store.GetEmptyBlockStats()
	if err != nil {
		return fmt.Errorf("failed to get empty block stats: %v", err)
	}

	e.metrics.ClientCount.Reset()
	e.metrics.MinerCount.Reset()
	e.metrics.EmptyBlocksByValidator.Reset()

	// Update client metrics
	for client, versions := range clientStats {
		for version, validators := range versions {
			for validator, count := range validators {
				e.metrics.ClientCount.WithLabelValues(client, version, validator).Set(float64(count))
			}
		}
	}

	// Update miner metrics
	for miner, clients := range minerStats {
		for client, validators := range clients {
			for validator, count := range validators {
				e.metrics.MinerCount.WithLabelValues(miner, client, validator).Set(float64(count))
			}
		}
	}

	// Update empty block metrics
	for validator, count := range emptyBlockStats {
		e.metrics.EmptyBlocksByValidator.WithLabelValues(validator).Set(float64(count))
	}

	return nil
}

func (e *Exporter) processBlock(number uint64) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	block, err := e.client.BlockByNumber(ctx, big.NewInt(int64(number)))
	if err != nil {
		return fmt.Errorf("failed to get block: %v", err)
	}
	if block == nil {
		return fmt.Errorf("received nil block")
	}

	extraDataHex := hex.EncodeToString(block.Extra())
	minerAddress := normalizeAddress(block.Coinbase().Hex())

	client, version := e.detector.DetectClient(extraDataHex, "")
	validatorName := types.ValidatorNames[minerAddress]
	if validatorName == "" {
		validatorName = fmt.Sprintf("Unknown Validator (%s)", minerAddress)
	}

	txCount := len(block.Transactions())
	isEmpty := txCount == 0

	blockInfo := types.BlockInfo{
		Number:        number,
		Miner:         minerAddress,
		ValidatorName: validatorName,
		Client:        client,
		Version:       version,
		ExtraDataHex:  extraDataHex,
		TxCount:       txCount,
		IsEmpty:       isEmpty,
		Timestamp:     time.Now(),
	}

	if err := e.store.StoreBlock(blockInfo); err != nil {
		return fmt.Errorf("failed to store block: %v", err)
	}

	emoji := types.ClientEmojis[client]
	if emoji == "" {
		emoji = "❓"
	}

	blockType := "📦"
	if isEmpty {
		blockType = "🕸️ Empty"
	} else {
		blockType = fmt.Sprintf("📦 Txs: %d", txCount)
	}

	log.Printf("Block %d | %s %s | %s | Validator: %s | Miner: %s | Version: %s",
		number, emoji, client, blockType, validatorName, minerAddress, version)

	e.metrics.BlockProcessed.Inc()
	e.metrics.LastBlock.Set(float64(number))

	if isEmpty {
		e.metrics.EmptyBlocks.Inc()
	} else {
		e.metrics.TransactionCount.Add(float64(txCount))
	}

	return nil
}

func (e *Exporter) processBlockRange(ctx context.Context) error {
	e.config.Mu.RLock()
	currentBlock := e.config.CurrentBlock
	e.config.Mu.RUnlock()

	latestBlock, err := e.client.BlockNumber(ctx)
	if err != nil {
		return fmt.Errorf("failed to get latest block: %v", err)
	}

	if currentBlock > latestBlock {
		log.Printf("⚠️ Current block (%d) is ahead of latest block (%d), resetting to latest block",
			currentBlock, latestBlock)
		currentBlock = latestBlock
		e.config.Mu.Lock()
		e.config.CurrentBlock = currentBlock
		e.config.Mu.Unlock()
	}

	if currentBlock >= latestBlock {
		return nil
	}

	endBlock := currentBlock + e.batchSize
	if endBlock > latestBlock {
		endBlock = latestBlock
	}

	log.Printf("🔍 Processing blocks %d to %d (Latest: %d, Batch size: %d)",
		currentBlock, endBlock, latestBlock, e.batchSize)

	successCount := uint64(0)
	for blockNum := currentBlock; blockNum <= endBlock; blockNum++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			if err := e.processBlock(blockNum); err != nil {
				log.Printf("❌ Error processing block %d: %v", blockNum, err)
				continue
			}
			successCount++
		}
	}

	if successCount > 0 {
		if err := e.updateMetrics(); err != nil {
			log.Printf("⚠️ Error updating metrics: %v", err)
		}
	}

	e.config.Mu.Lock()
	e.config.CurrentBlock = endBlock + 1
	e.config.Mu.Unlock()

	log.Printf("✅ Processed blocks %d to %d (Success: %d)", currentBlock, endBlock, successCount)
	return nil
}

func (e *Exporter) Run(ctx context.Context) {
	log.Printf("🚀 Starting block processor with batch size %d", e.batchSize)

	log.Printf("Starting initial block processing...")
	if err := e.processBlockRange(ctx); err != nil {
		log.Printf("❌ Error in initial block processing: %v", err)
	}

	ticker := time.NewTicker(time.Second * 2)
	defer ticker.Stop()

	log.Printf("Starting periodic block processing...")
	for {
		select {
		case <-ctx.Done():
			log.Printf("🛑 Shutting down block processor")
			return
		case <-ticker.C:
			if err := e.processBlockRange(ctx); err != nil {
				log.Printf("❌ Error processing block range: %v", err)
				time.Sleep(time.Second * 1)
			}
		}
	}
}

func NewExporter(rpcURL string, dbConfig types.DBConfig, startBlock uint64, batchSize uint64) (*Exporter, error) {
	rpcClient, err := ethRpc.Dial(rpcURL)
	if err != nil {
		return nil, err
	}

	client := ethclient.NewClient(rpcClient)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	latestBlock, err := client.BlockNumber(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get latest block: %v", err)
	}

	store, err := store.NewStore(dbConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize store: %v", err)
	}

	metrics := metrics.NewMetrics()
	metrics.Register()

	lastProcessed, err := store.GetLastProcessedBlock()
	if err != nil {
		log.Printf("⚠️ Could not get last processed block: %v", err)
		lastProcessed = startBlock
	}

	var initialBlock uint64
	if startBlock > 0 {
		initialBlock = startBlock
		log.Printf("🎯 Using specified start block: %d", startBlock)
	} else if lastProcessed > 0 {
		initialBlock = lastProcessed
		log.Printf("🔄 Resuming from last processed block: %d", lastProcessed)
	} else {
		initialBlock = 0
		log.Printf("⚠️ Starting from genesis block")
	}

	if initialBlock > latestBlock {
		log.Printf("⚠️ Start block (%d) is beyond latest block (%d), using latest block",
			initialBlock, latestBlock)
		initialBlock = latestBlock
	}

	return &Exporter{
		client:    client,
		rpcClient: rpcClient,
		store:     store,
		metrics:   metrics,
		batchSize: batchSize,
		detector:  detector.NewClientDetector(),
		config: &types.Config{
			StartBlock:   startBlock,
			CurrentBlock: initialBlock,
		},
	}, nil
}

func main() {
	rpcURL := flag.String("rpc", "http://localhost:8545", "Ethereum RPC URL")
	dbDriver := flag.String("db-driver", "sqlite3", "Database driver (sqlite3 or mysql)")
	dbDSN := flag.String("db-dsn", "eth_clients.db", "Database DSN (file path for SQLite, connection string for MySQL)")
	listenAddr := flag.String("listen", ":9090", "Metrics listen address")
	validatorsFile := flag.String("validators", "validators.yaml", "Path to validators mapping YAML file")
	startBlock := flag.Uint64("start-block", 0, "Starting block number (0 for genesis)")
	batchSize := flag.Uint64("batch-size", 10, "Number of blocks to process in each batch")
	flag.Parse()

	// Load validators
	data, err := os.ReadFile(*validatorsFile)
	if err != nil {
		log.Printf("⚠️ Warning: Could not read validator file: %v", err)
	} else {
		var validatorMap map[string]string
		if err := yaml.Unmarshal(data, &validatorMap); err != nil {
			log.Printf("⚠️ Warning: Could not parse validator YAML: %v", err)
		} else {
			for addr, name := range validatorMap {
				normalizedAddr := normalizeAddress(addr)
				types.ValidatorNames[normalizedAddr] = name
				log.Printf("📝 Loaded validator: %s -> %s", normalizedAddr, name)
			}
		}
	}

	// Test RPC connection
	log.Printf("Testing RPC connection...")
	rpcClient, err := ethRpc.Dial(*rpcURL)
	if err != nil {
		log.Fatalf("❌ Failed to connect to RPC: %v", err)
	}
	client := ethclient.NewClient(rpcClient)

	testCtx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	latestBlock, err := client.BlockNumber(testCtx)
	if err != nil {
		log.Fatalf("❌ Failed to get latest block: %v", err)
	}
	log.Printf("✅ Successfully connected to RPC. Latest block: %d", latestBlock)

	dbConfig := types.DBConfig{
		Driver: *dbDriver,
		DSN:    *dbDSN,
	}

	exporter, err := NewExporter(*rpcURL, dbConfig, *startBlock, *batchSize)
	if err != nil {
		log.Fatalf("❌ Failed to create exporter: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go exporter.Run(ctx)

	http.Handle("/metrics", promhttp.Handler())
	log.Printf("🚀 Starting metrics server on %s", *listenAddr)

	if err := http.ListenAndServe(*listenAddr, nil); err != nil {
		log.Fatalf("❌ Error starting metrics server: %v", err)
	}
}
