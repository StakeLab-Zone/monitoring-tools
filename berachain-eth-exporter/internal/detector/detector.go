package detector

import (
	"encoding/hex"
	"log"
	"regexp"
	"strings"
)

type ClientDetector struct {
	patterns map[string]struct {
		signatures []string
		versions   []*regexp.Regexp
	}
}

func NewClientDetector() *ClientDetector {
	return &ClientDetector{
		patterns: map[string]struct {
			signatures []string
			versions   []*regexp.Regexp
		}{
			"Geth": {
				signatures: []string{"go-ethereum", "geth", "؃geth"},
				versions: []*regexp.Regexp{
					regexp.MustCompile(`(?i)geth/v?([\d\.]+)`),
					regexp.MustCompile(`(?i)go([\d\.]+)`),
				},
			},
			"RETH": {
				signatures: []string{"reth"},
				versions: []*regexp.Regexp{
					regexp.MustCompile(`(?i)reth/v?([\d\.]+)`),
					regexp.MustCompile(`(?i)reth/(v[\d\.]+)`),
				},
			},
			"Nethermind": {
				signatures: []string{"nethermind", "nethm"},
				versions: []*regexp.Regexp{
					regexp.MustCompile(`(?i)nethermind/v?([\d\.]+)`),
					regexp.MustCompile(`(?i)nethermind_([\d\.]+)`),
					regexp.MustCompile(`(?i)nethm/([\d\.]+)`),
				},
			},
			"Erigon": {
				signatures: []string{"erigon", "thorax/erigon", "erigontech/erigon"},
				versions: []*regexp.Regexp{
					regexp.MustCompile(`(?i)erigon/v?([\d\.]+)`),
					regexp.MustCompile(`(?i)erigon_([\d\.]+)`),
					regexp.MustCompile(`(?i)thorax/erigon:v([\d\.]+)`),
					regexp.MustCompile(`(?i)erigontech/erigon:v([\d\.]+)`),
				},
			},
			"Besu": {
				signatures: []string{"besu", "hyperledger"},
				versions: []*regexp.Regexp{
					regexp.MustCompile(`(?i)besu/v?([\d\.]+)`),
					regexp.MustCompile(`(?i)hyperledger-besu/([\d\.]+)`),
				},
			},
			"EthereumJS": {
				signatures: []string{"ethereumjs", "ethereum-js"},
				versions: []*regexp.Regexp{
					regexp.MustCompile(`(?i)ethereumjs/v?([\d\.]+)`),
					regexp.MustCompile(`(?i)ethereum-js/([\d\.]+)`),
				},
			},
		},
	}
}

func (d *ClientDetector) DetectClient(extraDataHex string, extraDataText string) (string, string) {
	// Try to decode hex first
	var searchTexts []string

	// Add the original data
	searchTexts = append(searchTexts, strings.ToLower(extraDataText))

	// Try hex decoding
	if decoded, err := hex.DecodeString(strings.TrimPrefix(extraDataHex, "0x")); err == nil {
		decodedText := strings.ToLower(string(decoded))
		searchTexts = append(searchTexts, decodedText)
		log.Printf("🔍 Decoded hex: '%s'", decodedText)
	}

	// Add raw hex
	searchTexts = append(searchTexts, strings.ToLower(extraDataHex))

	log.Printf("🔍 Searching in texts: %v", searchTexts)

	// Check each search text
	for _, searchText := range searchTexts {
		for client, patterns := range d.patterns {
			for _, sig := range patterns.signatures {
				if strings.Contains(searchText, strings.ToLower(sig)) {
					log.Printf("✅ Found signature '%s' for client '%s'", sig, client)

					// Try to find version
					for _, verPattern := range patterns.versions {
						if match := verPattern.FindStringSubmatch(searchText); len(match) > 1 {
							log.Printf("✅ Detected %s version %s", client, match[1])
							return client, match[1]
						}
					}

					log.Printf("✅ Detected %s (no version)", client)
					return client, ""
				}
			}
		}
	}

	// Common hex signatures
	hexSignatures := map[string]string{
		"67657468":               "Geth", // "geth" in hex
		"676f2d657468657265756d": "Geth", // "go-ethereum" in hex
	}

	for hexSig, client := range hexSignatures {
		if strings.Contains(strings.ToLower(extraDataHex), hexSig) {
			log.Printf("✅ Detected %s from hex signature", client)
			return client, ""
		}
	}

	// Log the raw data for debugging
	// log.Printf("❌ Unknown client data:")
	// log.Printf("  Hex: '%s'", extraDataHex)
	// log.Printf("  Text: '%s'", extraDataText)
	// log.Printf("  Decoded attempts: %v", searchTexts)

	return "Unknown", ""
}
