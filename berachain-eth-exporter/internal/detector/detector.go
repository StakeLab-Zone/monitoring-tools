package detector

import (
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/ethereum/go-ethereum/rlp"
)

type ClientDetector struct{}

func NewClientDetector() *ClientDetector {
	return &ClientDetector{}
}

func (d *ClientDetector) DetectClient(extraDataHex string, extraDataText string) (string, string) {
	// Clean the hex string
	cleanHex := strings.TrimPrefix(extraDataHex, "0x")

	// First, try to decode as ASCII string from hex
	if client, version := d.detectASCIIFromHex(cleanHex); client != "" {
		return client, version
	}

	// Then, try to decode as RLP
	if client, version := d.detectRLP(cleanHex); client != "" {
		return client, version
	}

	// If nothing found, return unknown
	return "Unknown", ""
}

func (d *ClientDetector) detectASCIIFromHex(hexStr string) (string, string) {
	// Try to decode hex as ASCII string
	decoded, err := hex.DecodeString(hexStr)
	if err != nil {
		return "", ""
	}

	decodedStr := string(decoded)

	// Look for pattern like "bera-geth/v1.4.0-rc2/linux"
	// We want the first two fields: client and version
	parts := strings.Split(decodedStr, "/")
	if len(parts) >= 2 {
		client := parts[0]
		version := parts[1]

		// Clean up version (remove 'v' prefix if present)
		version = strings.TrimPrefix(version, "v")

		return client, version
	}

	return "", ""
}

func (d *ClientDetector) detectRLP(hexStr string) (string, string) {
	decoded, err := hex.DecodeString(hexStr)
	if err != nil {
		return "", ""
	}

	var items []interface{}
	err = rlp.DecodeBytes(decoded, &items)
	if err != nil {
		return "", ""
	}

	var client, version string

	// If first field is a byte array of length >= 3, treat as version numbers and second as client
	if len(items) >= 2 {
		if versionBytes, ok := items[0].([]byte); ok && len(versionBytes) >= 3 {
			version = fmt.Sprintf("%d.%d.%d", versionBytes[0], versionBytes[1], versionBytes[2])
			if clientBytes, ok := items[1].([]byte); ok {
				client = string(clientBytes)
			} else if clientStr, ok := items[1].(string); ok {
				client = clientStr
			}
			return client, version
		}
	}

	// Otherwise, find first string/[]byte as client, second as version
	foundClient := false
	for _, item := range items {
		if !foundClient {
			if b, ok := item.([]byte); ok && len(b) > 0 {
				client = string(b)
				foundClient = true
				continue
			} else if s, ok := item.(string); ok && len(s) > 0 {
				client = s
				foundClient = true
				continue
			}
		} else {
			if b, ok := item.([]byte); ok && len(b) > 0 {
				version = string(b)
				break
			} else if s, ok := item.(string); ok && len(s) > 0 {
				version = s
				break
			}
		}
	}

	if client != "" {
		return client, version
	}

	return "", ""
}
