# Ethereum Block Exporter

Ethereum Block Exporter is a Go-based tool that monitors Ethereum blocks and exports metrics about mined blocks and validators. It uses Prometheus for metrics exposition and can be configured to monitor specific addresses or all validators.

## Features

    Monitor Ethereum blocks in real-time
    Track blocks mined by specific validators or all validators
    Export Prometheus metrics for mined blocks, empty blocks, and last block times
    Configurable via YAML files and command-line flags
    Periodic refresh of validator names
    Debug mode for verbose logging

## Prerequisites

Go 1.21 or higher
Access to an Ethereum node (e.g., Infura, Alchemy, or your own node)

## Installation

Clone the repository:

```bash
git clone https://github.com/StakeLab-Zone/monitoring-tools.git
cd ethereum-blocks-exporter
```

Install the required dependencies:
```bash
go get github.com/ethereum/go-ethereum/ethclient
go get github.com/prometheus/client_golang/prometheus
go get github.com/prometheus/client_golang/prometheus/promhttp
go get gopkg.in/yaml.v2
```

## Build the application:

`go build -o eth-block-exporter cmd/main.go`


## Configuration

Create a config.yaml file with your desired settings:

```yaml
eth_endpoint: "https://localhost:8545"
addresses:
  - "0x1234567890123456789012345678901234567890"
  - "0x0987654321098765432109876543210987654321"
start_block: 1000000
max_blocks: 1000
monitor_all: false
```
Create a validator_names.yaml file with validator names:

```yaml
"0x1234567890123456789012345678901234567890": "Validator1"
"0x0987654321098765432109876543210987654321": "Validator2"
```

## Usage

Run the Ethereum Block Exporter with the following command:

`./main -config=config.yaml -validator-names=validator_names.yaml`

Command-line Flags

`-config`: Path to the configuration file (default: "config.yaml")

`-validator-names`: Path to the validator names file (default: "validator_names.yaml")

`-metrics-port`: Port for exposing metrics (default: 8080)

`-start-block`: Block number to start processing from (overrides config file)

`-max-blocks`: Maximum number of blocks to process per cycle (overrides config file)

`-monitor-all`: Monitor all validators instead of specific addresses (overrides config file)

`-quiet`: Run in quiet mode with minimal logging

`-debug`: Run in debug mode with extra logging

## Metrics

The exporter provides the following Prometheus metrics:

`eth_blocks_mined_total`: Total number of Ethereum blocks mined by address/validator

`eth_empty_blocks_mined_total`: Total number of empty Ethereum blocks mined by address/validator

`eth_last_block_time`: Timestamp of the last mined block by address/validator

`eth_last_check_time`: Timestamp of the last check for new blocks

Access metrics at http://localhost:8080/metrics (or the port specified with -metrics-port).

## Troubleshooting

If you encounter issues with validator name resolution:

Enable debug mode using the -debug flag
Check the debug output for errors in loading or parsing the YAML files
Verify that the addresses in validator_names.yaml match those in the blocks (case-insensitive)
Ensure the YAML files are properly formatted and accessible to the application

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.
