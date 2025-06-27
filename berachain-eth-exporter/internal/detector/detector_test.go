package detector

import (
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
		{
			name:            "ASCII string encoded in HEX - reth example 1",
			extraDataHex:    "0x726574682f76312e312e352f6c696e7578",
			extraDataText:   "",
			expectedClient:  "reth",
			expectedVersion: "1.1.5",
		},
		{
			name:            "ASCII string encoded in HEX - reth example 2",
			extraDataHex:    "0x726574682f76312e302e382f6c696e7578",
			extraDataText:   "",
			expectedClient:  "reth",
			expectedVersion: "1.0.8",
		},
		{
			name:            "RLP encoded with 4 fields - geth example 1",
			extraDataHex:    "0xd883010e0d846765746888676f312e32332e33856c696e7578",
			extraDataText:   "",
			expectedClient:  "geth",
			expectedVersion: "1.14.13",
		},
		{
			name:            "RLP encoded with 4 fields - geth example 2",
			extraDataHex:    "0xd883010f0b846765746888676f312e32342e32856c696e7578",
			extraDataText:   "",
			expectedClient:  "geth",
			expectedVersion: "1.15.11",
		},
		{
			name:            "Empty extraData",
			extraDataHex:    "0x",
			extraDataText:   "",
			expectedClient:  "Unknown",
			expectedVersion: "",
		},
		{
			name:            "Invalid hex data",
			extraDataHex:    "0xinvalid",
			extraDataText:   "",
			expectedClient:  "Unknown",
			expectedVersion: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, version := detector.DetectClient(tt.extraDataHex, tt.extraDataText)

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
			name:            "reth example 1",
			hexStr:          "726574682f76312e312e352f6c696e7578",
			expectedClient:  "reth",
			expectedVersion: "1.1.5",
		},
		{
			name:            "reth example 2",
			hexStr:          "726574682f76312e302e382f6c696e7578",
			expectedClient:  "reth",
			expectedVersion: "1.0.8",
		},
		{
			name:            "invalid hex",
			hexStr:          "invalid",
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
			name:            "geth RLP example 1",
			hexStr:          "d883010e0d846765746888676f312e32332e33856c696e7578",
			expectedClient:  "geth",
			expectedVersion: "1.14.13",
		},
		{
			name:            "geth RLP example 2",
			hexStr:          "d883010f0b846765746888676f312e32342e32856c696e7578",
			expectedClient:  "geth",
			expectedVersion: "1.15.11",
		},
		{
			name:            "invalid RLP",
			hexStr:          "invalid",
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
