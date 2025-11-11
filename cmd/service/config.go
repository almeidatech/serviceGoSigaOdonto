// cmd/service/config.go
package main

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
)

type Config struct {
	LogLevel    string `json:"logLevel"`
	LogPath     string `json:"logPath"`
	UpdateURL   string `json:"updateUrl"`
	Development struct {
		Enabled     bool `json:"enabled"`
		AutoRestart bool `json:"autoRestart"`
		DebugLog    bool `json:"debugLog"`
	} `json:"development"`
	WebSocket struct {
		Enabled           bool   `json:"enabled"`
		ServerURL         string `json:"serverUrl"`
		AuthToken         string `json:"authToken"`
		Port              int    `json:"port"`
		ReconnectInterval int    `json:"reconnectInterval"` // em segundos
		PingInterval      int    `json:"pingInterval"`      // em segundos
		ClinicID          string `json:"clinicId"`
		APIKey            string `json:"apiKey"`
		InstanceID        string `json:"instanceId"`
		UseSSL            bool   `json:"useSSL"`
	} `json:"websocket"`
	Database struct {
		DSN      string `json:"dsn"`
		MaxConns int    `json:"maxConns"`
	} `json:"database"`
}

func validateConfig(config *Config) error {
	if config.Database.MaxConns <= 0 {
		config.Database.MaxConns = 10 // valor padrão
	}

	if config.LogPath == "" {
		return fmt.Errorf("caminho do log não pode estar vazio")
	}

	// Garante que o diretório do log existe
	logDir := filepath.Dir(config.LogPath)
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return fmt.Errorf("erro ao criar diretório de log: %v", err)
	}

	// Validação das configurações do WebSocket
	if config.WebSocket.Enabled {
		if config.WebSocket.ServerURL == "" {
			return fmt.Errorf("URL do servidor WebSocket não pode estar vazia quando WebSocket está habilitado")
		}
		if config.WebSocket.ReconnectInterval <= 0 {
			config.WebSocket.ReconnectInterval = 30 // 30 segundos padrão
		}
		if config.WebSocket.PingInterval <= 0 {
			config.WebSocket.PingInterval = 30 // 30 segundos padrão
		}

		// Novas validações de segurança
		if config.WebSocket.ClinicID == "" {
			return fmt.Errorf("clinic_id não pode estar vazio quando WebSocket está habilitado")
		}
		if config.WebSocket.APIKey == "" {
			return fmt.Errorf("api_key não pode estar vazio quando WebSocket está habilitado")
		}
		if config.WebSocket.InstanceID == "" {
			// Gerar um ID de instância único se não fornecido
			config.WebSocket.InstanceID = generateInstanceID()
		}
		// Force SSL em produção
		if !config.Development.Enabled && !config.WebSocket.UseSSL {
			return fmt.Errorf("SSL é obrigatório em ambiente de produção")
		}
	}

	return nil
}

func loadConfig(exePath string) error {
	configPath := filepath.Join(filepath.Dir(exePath), "config.json")

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		defaultConfig := Config{
			LogLevel:  "info",
			LogPath:   filepath.Join(filepath.Dir(exePath), "olmeda-service.log"),
			UpdateURL: "http://localhost:8080/update",
			Development: struct {
				Enabled     bool `json:"enabled"`
				AutoRestart bool `json:"autoRestart"`
				DebugLog    bool `json:"debugLog"`
			}{
				Enabled:     true,
				AutoRestart: true,
				DebugLog:    true,
			},
			WebSocket: struct {
				Enabled           bool   `json:"enabled"`
				ServerURL         string `json:"serverUrl"`
				AuthToken         string `json:"authToken"`
				Port              int    `json:"port"`
				ReconnectInterval int    `json:"reconnectInterval"`
				PingInterval      int    `json:"pingInterval"`
				ClinicID          string `json:"clinicId"`
				APIKey            string `json:"apiKey"`
				InstanceID        string `json:"instanceId"`
				UseSSL            bool   `json:"useSSL"`
			}{
				Enabled:           true,
				ServerURL:         "wss://odonto-olmeda-production.up.railway.app/ws",
				AuthToken:         "",
				Port:              8081,
				ReconnectInterval: 30,
				PingInterval:      30,
				ClinicID:          "",
				APIKey:            "",
				InstanceID:        "",
				UseSSL:            true,
			},
			Database: struct {
				DSN      string `json:"dsn"`
				MaxConns int    `json:"maxConns"`
			}{
				DSN:      "Driver={Microsoft Access Driver (*.mdb, *.accdb)};DBQ=D:\\teste\\odontoDB.mdb;DefaultDir=D:\\teste;",
				MaxConns: 10,
			},
		}

		file, err := os.Create(configPath)
		if err != nil {
			return err
		}
		defer file.Close()

		encoder := json.NewEncoder(file)
		encoder.SetIndent("", "    ")
		if err := encoder.Encode(defaultConfig); err != nil {
			return err
		}

		configuration = defaultConfig
		return nil
	}

	file, err := os.Open(configPath)
	if err != nil {
		return err
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&configuration); err != nil {
		return err
	}

	return validateConfig(&configuration)
}

// Função auxiliar para debug
func debugLog(format string, v ...interface{}) {
	if configuration.Development.Enabled && configuration.Development.DebugLog {
		log.Printf("[DEBUG] "+format, v...)
	}
}

// Nova função para gerar InstanceID
func generateInstanceID() string {
	// Gera um UUID v4
	uuid := make([]byte, 16)
	rand.Read(uuid)
	uuid[6] = (uuid[6] & 0x0f) | 0x40 // Version 4
	uuid[8] = (uuid[8] & 0x3f) | 0x80 // Variant is 10

	return fmt.Sprintf("%x-%x-%x-%x-%x",
		uuid[0:4], uuid[4:6], uuid[6:8], uuid[8:10], uuid[10:])
}
