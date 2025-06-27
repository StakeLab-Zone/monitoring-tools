package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	defaultRPCURL = "http://10.147.18.191:41003/"
	batchSize     = 50 // Number of blocks to request in each batch
)

var (
	rpcURL = getEnvOrDefault("EL_ETHRPC_URL", defaultRPCURL)
	client = &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 100,
			IdleConnTimeout:     90 * time.Second,
		},
	}
)

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getCurrentBlock() (uint64, error) {
	payload := `{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}`
	resp, err := client.Post(rpcURL, "application/json", strings.NewReader(payload))
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	var result struct {
		Result string `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, err
	}
	height, _ := new(big.Int).SetString(result.Result[2:], 16)
	return height.Uint64(), nil
}

func getBlockExtraDataBatch(blockNums []uint64) (map[uint64]string, error) {
	if len(blockNums) == 0 {
		return nil, nil
	}

	// Create batch request
	var requests []map[string]interface{}
	for i, blockNum := range blockNums {
		blockHex := fmt.Sprintf("0x%x", blockNum)
		requests = append(requests, map[string]interface{}{
			"jsonrpc": "2.0",
			"method":  "eth_getBlockByNumber",
			"params":  []interface{}{blockHex, false},
			"id":      i + 1,
		})
	}

	payload, err := json.Marshal(requests)
	if err != nil {
		return nil, err
	}

	resp, err := client.Post(rpcURL, "application/json", bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var results []struct {
		Result struct {
			ExtraData string `json:"extraData"`
		} `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		return nil, err
	}

	extraDataMap := make(map[uint64]string)
	for i, result := range results {
		if i < len(blockNums) {
			extraDataMap[blockNums[i]] = result.Result.ExtraData
		}
	}

	return extraDataMap, nil
}

func parseRange(args []string) (uint64, uint64, error) {
	if len(args) == 0 {
		// Default: scan all blocks from 1 to current
		height, err := getCurrentBlock()
		if err != nil {
			return 0, 0, fmt.Errorf("failed to get current block: %v", err)
		}
		return 1, height, nil
	}

	if len(args) == 1 {
		// Single argument: start from block 1 to specified block
		end, err := strconv.ParseUint(args[0], 10, 64)
		if err != nil {
			return 0, 0, fmt.Errorf("invalid end block: %v", err)
		}
		return 1, end, nil
	}

	// Two arguments: start and end blocks
	start, err := strconv.ParseUint(args[0], 10, 64)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid start block: %v", err)
	}

	end, err := strconv.ParseUint(args[1], 10, 64)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid end block: %v", err)
	}

	if start > end {
		return 0, 0, fmt.Errorf("start block (%d) cannot be greater than end block (%d)", start, end)
	}

	return start, end, nil
}

func main() {
	// Parse command line arguments for block range
	startBlock, endBlock, err := parseRange(os.Args[1:])
	if err != nil {
		fmt.Printf("Usage: %s [start_block] [end_block]\n", os.Args[0])
		fmt.Printf("  No args: scan all blocks from 1 to current\n")
		fmt.Printf("  One arg: scan from block 1 to specified block\n")
		fmt.Printf("  Two args: scan from start_block to end_block\n")
		fmt.Printf("Error: %v\n", err)
		return
	}

	totalBlocks := endBlock - startBlock + 1
	fmt.Printf("Scanning blocks %d to %d (%d total blocks)\n", startBlock, endBlock, totalBlocks)
	fmt.Printf("Using RPC URL: %s\n", rpcURL)

	seen := make(map[string]struct{})
	start := time.Now()
	blocksProcessed := uint64(0)

	// Process blocks linearly in batches
	for currentBlock := startBlock; currentBlock <= endBlock; currentBlock += uint64(batchSize) {
		batchEnd := currentBlock + uint64(batchSize) - 1
		if batchEnd > endBlock {
			batchEnd = endBlock
		}

		// Generate batch of consecutive block numbers
		var batch []uint64
		for i := currentBlock; i <= batchEnd; i++ {
			batch = append(batch, i)
		}

		extraDataMap, err := getBlockExtraDataBatch(batch)
		if err != nil {
			fmt.Printf("Error fetching batch %d-%d: %v\n", currentBlock, batchEnd, err)
			time.Sleep(100 * time.Millisecond)
			continue
		}

		for blockNum, extraData := range extraDataMap {
			if extraData == "" {
				continue
			}
			if _, exists := seen[extraData]; !exists {
				seen[extraData] = struct{}{}
				fmt.Printf("Block %d: %s\n", blockNum, extraData)
			}
		}

		blocksProcessed += uint64(len(batch))

		// Progress reporting every 1000 blocks
		if blocksProcessed%1000 == 0 || currentBlock == endBlock {
			elapsed := time.Since(start)
			percent := float64(blocksProcessed) / float64(totalBlocks) * 100
			fmt.Printf("Progress: %d/%d (%.2f%%), %d unique extraData, elapsed: %s\n",
				blocksProcessed, totalBlocks, percent, len(seen), elapsed.Truncate(time.Second))
		}

		time.Sleep(5 * time.Millisecond)
	}

	fmt.Println("\nUnique extraData values:")
	for ed := range seen {
		fmt.Println(ed)
	}
	fmt.Printf("Done. Scanned %d blocks (%d to %d), found %d unique extraData. Total time: %s\n",
		totalBlocks, startBlock, endBlock, len(seen), time.Since(start).Truncate(time.Second))
}
