// internal/database/config.go
package database

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type Config struct {
	Database struct {
		DSN      string `json:"dsn"`
		MaxConns int    `json:"maxConns"`
	} `json:"database"`
}

func LoadConfig() (*Config, error) {
	// Primeiro tenta no diretório atual
	configPath := "config.json"

	// Se não encontrar no diretório atual, tenta no diretório do executável
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		exePath, err := os.Executable()
		if err != nil {
			return nil, fmt.Errorf("erro ao obter caminho do executável: %v", err)
		}
		configPath = filepath.Join(filepath.Dir(exePath), "config.json")
	}

	file, err := os.Open(configPath)
	if err != nil {
		return nil, fmt.Errorf("erro ao abrir arquivo de configuração em %s: %v", configPath, err)
	}
	defer file.Close()

	var config Config
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&config); err != nil {
		return nil, fmt.Errorf("erro ao decodificar configuração: %v", err)
	}

	return &config, nil
}
