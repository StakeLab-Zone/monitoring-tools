package config

import (
	"fmt"
	"log"
	"os"
	"strings"

	"eth-exporter/internal/types"

	"gopkg.in/yaml.v3"
)

func normalizeAddress(address string) string {
	addr := strings.ToLower(address)
	if !strings.HasPrefix(addr, "0x") {
		addr = "0x" + addr
	}
	return addr
}

func LoadValidatorNames(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read validator file: %v", err)
	}

	var validatorMap map[string]string
	if err := yaml.Unmarshal(data, &validatorMap); err != nil {
		return fmt.Errorf("failed to parse validator YAML: %v", err)
	}

	types.ValidatorNames = make(map[string]string)
	for addr, name := range validatorMap {
		normalizedAddr := normalizeAddress(addr)
		types.ValidatorNames[normalizedAddr] = name
		log.Printf("📝 Loaded validator: %s -> %s", normalizedAddr, name)
	}

	return nil
}

func DumpValidatorMap() {
	log.Printf("🔍 Dumping validator map contents:")
	for addr, name := range types.ValidatorNames {
		log.Printf("Address: %s -> Name: %s", addr, name)
	}
}
