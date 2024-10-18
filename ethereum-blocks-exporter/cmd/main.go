package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"math/big"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"gopkg.in/yaml.v2"
)

type Config struct {
	EthEndpoint string   `yaml:"eth_endpoint"`
	Addresses   []string `yaml:"addresses"`
	StartBlock  uint64   `yaml:"start_block"`
	MaxBlocks   uint64   `yaml:"max_blocks"`
	MonitorAll  bool     `yaml:"monitor_all"`
}

var (
	configFile          string
	validatorNamesFile  string
	metricsPort         int
	startBlock          uint64
	maxBlocks           uint64
	monitorAll          bool
	quietMode           bool
	debugMode           bool
	config              Config
	validatorNames      map[string]string
	validatorNamesMutex sync.RWMutex
	blocksMined         *prometheus.CounterVec
	emptyBlocks         *prometheus.CounterVec
	lastBlockTime       *prometheus.GaugeVec
	lastCheckTime       prometheus.Gauge
)

func init() {
	flag.StringVar(&configFile, "config", "config.yaml", "Path to configuration file")
	flag.StringVar(&validatorNamesFile, "validator-names", "validator_names.yaml", "Path to validator names file")
	flag.IntVar(&metricsPort, "metrics-port", 8080, "Port for exposing metrics")
	flag.Uint64Var(&startBlock, "start-block", 0, "Block number to start processing from")
	flag.Uint64Var(&maxBlocks, "max-blocks", 1000, "Maximum number of blocks to process per cycle")
	flag.BoolVar(&monitorAll, "monitor-all", false, "Monitor all validators instead of specific addresses")
	flag.BoolVar(&quietMode, "quiet", false, "Run in quiet mode with minimal logging")
	flag.BoolVar(&debugMode, "debug", false, "Run in debug mode with extra logging")
	flag.Parse()

	blocksMined = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "eth_blocks_mined_total",
			Help: "Total number of Ethereum blocks mined by address/validator",
		},
		[]string{"address", "name"},
	)
	emptyBlocks = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "eth_empty_blocks_mined_total",
			Help: "Total number of empty Ethereum blocks mined by address/validator",
		},
		[]string{"address", "name"},
	)
	lastBlockTime = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "eth_last_block_time",
			Help: "Timestamp of the last mined block by address/validator",
		},
		[]string{"address", "name"},
	)
	lastCheckTime = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "eth_last_check_time",
			Help: "Timestamp of the last check for new blocks",
		},
	)

	prometheus.MustRegister(blocksMined, emptyBlocks, lastBlockTime, lastCheckTime)
}

func main() {
	logInfo("🚀 Starting Ethereum Block Exporter")

	if err := loadConfig(); err != nil {
		log.Fatalf("❌ Error loading configuration: %v", err)
	}

	if err := loadValidatorNames(); err != nil {
		log.Fatalf("❌ Error loading validator names: %v", err)
	}

	client, err := ethclient.Dial(config.EthEndpoint)
	if err != nil {
		log.Fatalf("❌ Failed to connect to the Ethereum client: %v", err)
	}

	go monitorBlocks(client)
	go refreshValidatorNames()

	http.Handle("/metrics", promhttp.Handler())
	http.HandleFunc("/health", healthCheckHandler)

	logInfo("📊 Metrics server listening on :%d", metricsPort)
	if err := http.ListenAndServe(fmt.Sprintf(":%d", metricsPort), nil); err != nil {
		log.Fatalf("❌ Error starting HTTP server: %v", err)
	}
}

func loadConfig() error {
	logDebug("Loading configuration from: %s", configFile)
	file, err := os.ReadFile(configFile)
	if err != nil {
		return fmt.Errorf("error reading config file: %v", err)
	}

	if err := yaml.Unmarshal(file, &config); err != nil {
		return fmt.Errorf("error parsing config file: %v", err)
	}

	// Override config with command-line flags if provided
	if startBlock != 0 {
		config.StartBlock = startBlock
	}
	if maxBlocks != 0 {
		config.MaxBlocks = maxBlocks
	}
	if monitorAll {
		config.MonitorAll = monitorAll
	}

	// Ensure addresses are in the correct format (lowercase)
	for i, addr := range config.Addresses {
		config.Addresses[i] = strings.ToLower(addr)
	}

	logInfo("📋 Loaded configuration: ETH Endpoint: %s, Monitor All: %v, Start Block: %d, Max Blocks: %d",
		config.EthEndpoint, config.MonitorAll, config.StartBlock, config.MaxBlocks)
	if !config.MonitorAll {
		logInfo("🔍 Monitoring addresses: %v", config.Addresses)
	}
	return nil
}

func loadValidatorNames() error {
	logDebug("Attempting to read validator names from: %s", validatorNamesFile)
	file, err := os.ReadFile(validatorNamesFile)
	if err != nil {
		return fmt.Errorf("error reading validator names file: %v", err)
	}
	logDebug("File content: %s", string(file))

	var newNames map[string]string
	if err := yaml.Unmarshal(file, &newNames); err != nil {
		return fmt.Errorf("error parsing validator names file: %v", err)
	}

	normalizedNames := make(map[string]string)
	for addr, name := range newNames {
		normalizedAddr := strings.ToLower(addr)
		normalizedNames[normalizedAddr] = name
		logDebug("Normalized validator: %s = %s", normalizedAddr, name)
	}

	validatorNamesMutex.Lock()
	validatorNames = normalizedNames
	validatorNamesMutex.Unlock()

	logInfo("📋 Loaded validator names for %d addresses", len(validatorNames))
	return nil
}

func refreshValidatorNames() {
	for {
		time.Sleep(2 * time.Minute)
		if err := loadValidatorNames(); err != nil {
			logError("⚠️ Error refreshing validator names: %v", err)
		} else {
			logInfo("🔄 Refreshed validator names")
		}
	}
}

func getValidatorName(address string) string {
	validatorNamesMutex.RLock()
	defer validatorNamesMutex.RUnlock()

	address = strings.ToLower(address)
	if name, ok := validatorNames[address]; ok {
		return name
	}
	logDebug("Validator name not found for address: %s", address)
	return "Unknown"
}

func monitorBlocks(client *ethclient.Client) {
	if config.MonitorAll {
		logInfo("👀 Monitoring blocks for all validators")
	} else {
		logInfo("👀 Monitoring blocks for addresses: %v", config.Addresses)
	}

	lastBlockNumber := config.StartBlock

	for {
		logDebug("🔍 Calling ETH endpoint to fetch latest block header")
		header, err := client.HeaderByNumber(context.Background(), nil)
		if err != nil {
			logError("⚠️ Error fetching latest block header: %v", err)
			updateLastCheckTime()
			time.Sleep(time.Minute)
			continue
		}

		currentBlockNumber := header.Number.Uint64()
		logDebug("📊 Current block number: %d, Last processed block: %d", currentBlockNumber, lastBlockNumber)

		updateLastCheckTime()

		endBlock := min(lastBlockNumber+config.MaxBlocks, currentBlockNumber)
		for blockNumber := lastBlockNumber + 1; blockNumber <= endBlock; blockNumber++ {
			logDebug("🔍 Calling ETH endpoint to fetch block %d", blockNumber)
			block, err := client.BlockByNumber(context.Background(), big.NewInt(int64(blockNumber)))
			if err != nil {
				logError("⚠️ Error fetching block %d: %v", blockNumber, err)
				continue
			}

			miner := strings.ToLower(block.Coinbase().Hex())
			logDebug("Looking up name for miner: %s", miner)
			validatorName := getValidatorName(miner)
			logDebug("⛏️ Block %d mined by %s (%s)", block.NumberU64(), miner, validatorName)

			if config.MonitorAll || containsAddress(config.Addresses, miner) {
				blocksMined.WithLabelValues(miner, validatorName).Inc()
				lastBlockTime.WithLabelValues(miner, validatorName).Set(float64(block.Time()))

				if len(block.Transactions()) == 0 {
					logDebug("💨 Empty block %d mined by %s (%s)", block.NumberU64(), miner, validatorName)
					emptyBlocks.WithLabelValues(miner, validatorName).Inc()
				}
			}
		}

		lastBlockNumber = endBlock
		logInfo("💤 Processed blocks up to %d. Waiting for 1 minute before next check", lastBlockNumber)
		time.Sleep(time.Minute)
	}
}

func updateLastCheckTime() {
	lastCheckTime.Set(float64(time.Now().Unix()))
}

func healthCheckHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

func min(a, b uint64) uint64 {
	if a < b {
		return a
	}
	return b
}

func containsAddress(addresses []string, address string) bool {
	for _, a := range addresses {
		if strings.EqualFold(a, address) {
			return true
		}
	}
	return false
}

func logInfo(format string, v ...interface{}) {
	log.Printf(format, v...)
}

func logDebug(format string, v ...interface{}) {
	if debugMode && !quietMode {
		log.Printf("[DEBUG] "+format, v...)
	}
}

func logError(format string, v ...interface{}) {
	log.Printf("[ERROR] "+format, v...)
}
