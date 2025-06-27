package detector

import (
	"fmt"
	"testing"
)

func TestDetectClient(t *testing.T) {
	detector := NewClientDetector()

	tests := []struct {
		name            string
		extraDataHex    string
		extraDataText   string
		expectedClient  string
		expectedVersion string
	}{
		// Reth clients - ASCII format (real blockchain data)
		{
			name:            "reth_v1.0.6_current_network",
			extraDataHex:    "0x726574682f76312e302e362f6c696e7578",
			extraDataText:   "",
			expectedClient:  "reth",
			expectedVersion: "1.0.6",
		},
		{
			name:            "reth_v1.0.8_historical",
			extraDataHex:    "0x726574682f76312e302e382f6c696e7578",
			extraDataText:   "",
			expectedClient:  "reth",
			expectedVersion: "1.0.8",
		},
		{
			name:            "reth_v1.1.1_historical",
			extraDataHex:    "0x726574682f76312e312e312f6c696e7578",
			extraDataText:   "",
			expectedClient:  "reth",
			expectedVersion: "1.1.1",
		},
		{
			name:            "reth_v1.1.2_historical",
			extraDataHex:    "0x726574682f76312e312e322f6c696e7578",
			extraDataText:   "",
			expectedClient:  "reth",
			expectedVersion: "1.1.2",
		},
		{
			name:            "reth_v1.1.3_historical",
			extraDataHex:    "0x726574682f76312e312e332f6c696e7578",
			extraDataText:   "",
			expectedClient:  "reth",
			expectedVersion: "1.1.3",
		},
		{
			name:            "reth_v1.1.5_historical",
			extraDataHex:    "0x726574682f76312e312e352f6c696e7578",
			extraDataText:   "",
			expectedClient:  "reth",
			expectedVersion: "1.1.5",
		},
		{
			name:            "reth_v1.2.0_historical",
			extraDataHex:    "0x726574682f76312e322e302f6c696e7578",
			extraDataText:   "",
			expectedClient:  "reth",
			expectedVersion: "1.2.0",
		},
		// Geth clients - RLP format (real blockchain data)
		{
			name:            "geth_v1.14.13_go1.23.3_historical",
			extraDataHex:    "0xd883010e0d846765746888676f312e32332e33856c696e7578",
			extraDataText:   "",
			expectedClient:  "geth",
			expectedVersion: "1.14.13",
		},
		{
			name:            "geth_v1.14.13_go1.23.1_historical",
			extraDataHex:    "0xd883010e0d846765746888676f312e32332e31856c696e7578",
			extraDataText:   "",
			expectedClient:  "geth",
			expectedVersion: "1.14.13",
		},
		{
			name:            "geth_v1.14.13_go1.23.5_historical",
			extraDataHex:    "0xd883010e0d846765746888676f312e32332e35856c696e7578",
			extraDataText:   "",
			expectedClient:  "geth",
			expectedVersion: "1.14.13",
		},
		{
			name:            "geth_v1.14.12_go1.22.9_historical",
			extraDataHex:    "0xd883010e0c846765746888676f312e32322e39856c696e7578",
			extraDataText:   "",
			expectedClient:  "geth",
			expectedVersion: "1.14.12",
		},
		{
			name:            "geth_v1.14.11_go1.22.10_historical",
			extraDataHex:    "0xd983010e0b846765746889676f312e32322e3130856c696e7578",
			extraDataText:   "",
			expectedClient:  "geth",
			expectedVersion: "1.14.11",
		},
		{
			name:            "geth_v1.15.11_go1.24.2_recent",
			extraDataHex:    "0xd883010f0b846765746888676f312e32342e32856c696e7578",
			extraDataText:   "",
			expectedClient:  "geth",
			expectedVersion: "1.15.11",
		},
		{
			name:            "geth_v1.15.11_go1.24.3_recent",
			extraDataHex:    "0xd883010f0b846765746888676f312e32342e33856c696e7578",
			extraDataText:   "",
			expectedClient:  "geth",
			expectedVersion: "1.15.11",
		},
		{
			name:            "geth_v1.15.10_go1.23.6_recent",
			extraDataHex:    "0xd883010f0a846765746888676f312e32332e36856c696e7578",
			extraDataText:   "",
			expectedClient:  "geth",
			expectedVersion: "1.15.10",
		},
		// Empty extraData (Reth v1.3.x behavior found in recent blocks)
		{
			name:            "empty_reth_v1.3.x_recent",
			extraDataHex:    "0x",
			extraDataText:   "",
			expectedClient:  "Reth",
			expectedVersion: "v1.3.x",
		},
		// Invalid/unknown cases
		{
			name:            "invalid_unknown",
			extraDataHex:    "0xinvalid",
			extraDataText:   "",
			expectedClient:  "Unknown",
			expectedVersion: "",
		},
		{
			name:            "malformed_hex",
			extraDataHex:    "0xzzzzzz",
			extraDataText:   "",
			expectedClient:  "Unknown",
			expectedVersion: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, version := detector.DetectClient(tt.extraDataHex, tt.extraDataText)

			// Print input and result in a cleaner format
			fmt.Printf("%-30s %s -> %s %s\n", tt.name, tt.extraDataHex, client, version)

			if client != tt.expectedClient {
				t.Errorf("DetectClient() client = %v, want %v", client, tt.expectedClient)
			}
			if version != tt.expectedVersion {
				t.Errorf("DetectClient() version = %v, want %v", version, tt.expectedVersion)
			}
		})
	}
}

func TestDetectASCIIFromHex(t *testing.T) {
	detector := NewClientDetector()

	tests := []struct {
		name            string
		hexStr          string
		expectedClient  string
		expectedVersion string
	}{
		{
			name:            "reth_v1.0.6_current",
			hexStr:          "726574682f76312e302e362f6c696e7578",
			expectedClient:  "reth",
			expectedVersion: "1.0.6",
		},
		{
			name:            "reth_v1.0.8_historical",
			hexStr:          "726574682f76312e302e382f6c696e7578",
			expectedClient:  "reth",
			expectedVersion: "1.0.8",
		},
		{
			name:            "reth_v1.1.5_historical",
			hexStr:          "726574682f76312e312e352f6c696e7578",
			expectedClient:  "reth",
			expectedVersion: "1.1.5",
		},
		{
			name:            "reth_v1.2.0_historical",
			hexStr:          "726574682f76312e322e302f6c696e7578",
			expectedClient:  "reth",
			expectedVersion: "1.2.0",
		},
		{
			name:            "invalid_hex",
			hexStr:          "invalid",
			expectedClient:  "",
			expectedVersion: "",
		},
		{
			name:            "non_reth_ascii",
			hexStr:          "67657468", // "geth" in hex
			expectedClient:  "",
			expectedVersion: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, version := detector.detectASCIIFromHex(tt.hexStr)

			if client != tt.expectedClient {
				t.Errorf("detectASCIIFromHex() client = %v, want %v", client, tt.expectedClient)
			}
			if version != tt.expectedVersion {
				t.Errorf("detectASCIIFromHex() version = %v, want %v", version, tt.expectedVersion)
			}
		})
	}
}

func TestDetectRLP(t *testing.T) {
	detector := NewClientDetector()

	tests := []struct {
		name            string
		hexStr          string
		expectedClient  string
		expectedVersion string
	}{
		{
			name:            "geth_v1.14.13_go1.23.3_historical",
			hexStr:          "d883010e0d846765746888676f312e32332e33856c696e7578",
			expectedClient:  "geth",
			expectedVersion: "1.14.13",
		},
		{
			name:            "geth_v1.14.12_go1.22.9_historical",
			hexStr:          "d883010e0c846765746888676f312e32322e39856c696e7578",
			expectedClient:  "geth",
			expectedVersion: "1.14.12",
		},
		{
			name:            "geth_v1.14.11_go1.22.10_historical",
			hexStr:          "d983010e0b846765746889676f312e32322e3130856c696e7578",
			expectedClient:  "geth",
			expectedVersion: "1.14.11",
		},
		{
			name:            "geth_v1.15.11_go1.24.2_recent",
			hexStr:          "d883010f0b846765746888676f312e32342e32856c696e7578",
			expectedClient:  "geth",
			expectedVersion: "1.15.11",
		},
		{
			name:            "geth_v1.15.11_go1.24.3_recent",
			hexStr:          "d883010f0b846765746888676f312e32342e33856c696e7578",
			expectedClient:  "geth",
			expectedVersion: "1.15.11",
		},
		{
			name:            "geth_v1.15.10_go1.23.6_recent",
			hexStr:          "d883010f0a846765746888676f312e32332e36856c696e7578",
			expectedClient:  "geth",
			expectedVersion: "1.15.10",
		},
		{
			name:            "invalid_rlp",
			hexStr:          "invalid",
			expectedClient:  "",
			expectedVersion: "",
		},
		{
			name:            "malformed_rlp",
			hexStr:          "d883010e0d8467657468", // truncated
			expectedClient:  "",
			expectedVersion: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, version := detector.detectRLP(tt.hexStr)

			if client != tt.expectedClient {
				t.Errorf("detectRLP() client = %v, want %v", client, tt.expectedClient)
			}
			if version != tt.expectedVersion {
				t.Errorf("detectRLP() version = %v, want %v", version, tt.expectedVersion)
			}
		})
	}
}
