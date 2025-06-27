package main

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"time"
)

const (
	rpcURL      = "http://10.147.18.191:40003/"
	sampleCount = 500000
	batchSize   = 10 // Number of blocks to request in each batch
)

var client = &http.Client{
	Timeout: 30 * time.Second,
	Transport: &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 100,
		IdleConnTimeout:     90 * time.Second,
	},
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

func randomUint64(max uint64) uint64 {
	nBig, _ := rand.Int(rand.Reader, big.NewInt(int64(max)))
	return nBig.Uint64()
}

func main() {
	height, err := getCurrentBlock()
	if err != nil {
		panic(err)
	}
	fmt.Printf("Current block height: %d\n", height)

	seen := make(map[string]struct{})
	start := time.Now()

	// Generate all random block numbers first
	var blockNums []uint64
	for i := 0; i < sampleCount; i++ {
		blockNums = append(blockNums, randomUint64(height-1)+1)
	}

	// Process in batches
	for i := 0; i < len(blockNums); i += batchSize {
		end := i + batchSize
		if end > len(blockNums) {
			end = len(blockNums)
		}

		batch := blockNums[i:end]
		extraDataMap, err := getBlockExtraDataBatch(batch)
		if err != nil {
			fmt.Printf("Error fetching batch starting at %d: %v\n", i, err)
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

		if i%1000 == 0 && i > 0 {
			elapsed := time.Since(start)
			percent := float64(i) / float64(sampleCount) * 100
			fmt.Printf("Progress: %d/%d (%.2f%%), %d unique extraData, elapsed: %s\n", i, sampleCount, percent, len(seen), elapsed.Truncate(time.Second))
		}

		time.Sleep(5 * time.Millisecond) // Reduced sleep time
	}

	fmt.Println("Unique extraData values:")
	for ed := range seen {
		fmt.Println(ed)
	}
	fmt.Printf("Done. Sampled %d blocks, found %d unique extraData. Total time: %s\n", sampleCount, len(seen), time.Since(start).Truncate(time.Second))
}
