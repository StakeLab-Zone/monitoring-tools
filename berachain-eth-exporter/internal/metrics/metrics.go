// internal/metrics/metrics.go
package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
)

// Metrics holds all Prometheus metrics for the application
type Metrics struct {
	// Counter metrics
	BlockProcessed   prometheus.Counter
	EmptyBlocks      prometheus.Counter
	TransactionCount prometheus.Counter

	// Gauge metrics
	LastBlock prometheus.Gauge

	// Vector metrics
	ClientCount            *prometheus.GaugeVec
	MinerCount             *prometheus.GaugeVec
	EmptyBlocksByClient    *prometheus.GaugeVec
	EmptyBlocksByMiner     *prometheus.GaugeVec
	EmptyBlocksByValidator *prometheus.GaugeVec
	BlockTimeGauge         *prometheus.GaugeVec
}

// NewMetrics creates and initializes a new Metrics instance
func NewMetrics() *Metrics {
	return &Metrics{
		// Simple counters
		BlockProcessed: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "eth_blocks_processed_total",
			Help: "Total number of blocks processed",
		}),
		EmptyBlocks: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "eth_empty_blocks_total",
			Help: "Total number of empty blocks processed",
		}),
		TransactionCount: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "eth_transactions_total",
			Help: "Total number of transactions processed",
		}),

		// Simple gauges
		LastBlock: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "eth_last_block_number",
			Help: "Number of the last block processed",
		}),

		// Vector gauges
		ClientCount: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "eth_client_blocks",
				Help: "Number of blocks by client, version, and validator",
			},
			[]string{"client", "version", "validator"},
		),
		MinerCount: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "eth_miner_blocks",
				Help: "Number of blocks by miner, client, and validator",
			},
			[]string{"miner", "client", "validator"},
		),
		EmptyBlocksByClient: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "eth_empty_blocks_by_client",
				Help: "Number of empty blocks by client",
			},
			[]string{"client", "version"},
		),
		EmptyBlocksByMiner: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "eth_empty_blocks_by_miner",
				Help: "Number of empty blocks by miner",
			},
			[]string{"miner"},
		),
		EmptyBlocksByValidator: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "eth_empty_blocks_by_validator",
				Help: "Number of empty blocks by validator",
			},
			[]string{"validator"},
		),
		BlockTimeGauge: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "eth_block_time_seconds",
				Help: "Time taken to process each block",
			},
			[]string{"client"},
		),
	}
}

// Register registers all metrics with the Prometheus client
func (m *Metrics) Register() {
	// Register counters
	prometheus.MustRegister(m.BlockProcessed)
	prometheus.MustRegister(m.EmptyBlocks)
	prometheus.MustRegister(m.TransactionCount)

	// Register gauges
	prometheus.MustRegister(m.LastBlock)

	// Register vector metrics
	prometheus.MustRegister(m.ClientCount)
	prometheus.MustRegister(m.MinerCount)
	prometheus.MustRegister(m.EmptyBlocksByClient)
	prometheus.MustRegister(m.EmptyBlocksByMiner)
	prometheus.MustRegister(m.EmptyBlocksByValidator)
	prometheus.MustRegister(m.BlockTimeGauge)
}

// ResetVectorMetrics resets all vector metrics
func (m *Metrics) ResetVectorMetrics() {
	m.ClientCount.Reset()
	m.MinerCount.Reset()
	m.EmptyBlocksByClient.Reset()
	m.EmptyBlocksByMiner.Reset()
	m.EmptyBlocksByValidator.Reset()
	m.BlockTimeGauge.Reset()
}

// UpdateBlockMetrics updates metrics for a single block
func (m *Metrics) UpdateBlockMetrics(blockNumber uint64, client, version, miner, validator string, isEmpty bool, txCount int, processingTime float64) {
	// Update basic counters
	m.BlockProcessed.Inc()
	m.LastBlock.Set(float64(blockNumber))

	if isEmpty {
		m.EmptyBlocks.Inc()
		m.EmptyBlocksByClient.WithLabelValues(client, version).Inc()
		m.EmptyBlocksByMiner.WithLabelValues(miner).Inc()
		m.EmptyBlocksByValidator.WithLabelValues(validator).Inc()
	} else {
		m.TransactionCount.Add(float64(txCount))
	}

	// Update processing time
	m.BlockTimeGauge.WithLabelValues(client).Set(processingTime)
}

// UpdateStats updates all aggregated statistics
func (m *Metrics) UpdateStats(
	clientStats map[string]map[string]map[string]int,
	minerStats map[string]map[string]map[string]int,
	emptyBlockStats map[string]int,
) {
	// Reset all vector metrics before updating
	m.ResetVectorMetrics()

	// Update client stats
	for client, versions := range clientStats {
		for version, validators := range versions {
			for validator, count := range validators {
				m.ClientCount.WithLabelValues(client, version, validator).Set(float64(count))
			}
		}
	}

	// Update miner stats
	for miner, clients := range minerStats {
		for client, validators := range clients {
			for validator, count := range validators {
				m.MinerCount.WithLabelValues(miner, client, validator).Set(float64(count))
			}
		}
	}

	// Update empty block stats
	for validator, count := range emptyBlockStats {
		m.EmptyBlocksByValidator.WithLabelValues(validator).Set(float64(count))
	}
}
