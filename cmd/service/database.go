// cmd/service/database.go
package main

import (
	"database/sql"
	"fmt"
	"sync"

	_ "github.com/alexbrainman/odbc"
)

var (
	db   *sql.DB
	once sync.Once
	mu   sync.RWMutex
)

// database.go - alterações principais
func InitDatabase() error {
	var initErr error
	once.Do(func() {
		debugLog("Iniciando conexão com o banco de dados...")
		debugLog("DSN configurado: %s", configuration.Database.DSN)

		db, initErr = sql.Open("odbc", configuration.Database.DSN)
		if initErr != nil {
			debugLog("Erro ao abrir conexão: %v", initErr)
			return
		}

		// Teste de conexão
		if initErr = db.Ping(); initErr != nil {
			debugLog("Erro no ping do banco: %v", initErr)
			db.Close()
			return
		}

		debugLog("Conexão com banco estabelecida com sucesso!")
	})
	return initErr
}

func CloseDatabase() {
	mu.Lock()
	defer mu.Unlock()

	if db != nil {
		debugLog("Fechando conexão com o banco de dados...")
		db.Close()
		db = nil
	}
}

func GetDB() *sql.DB {
	mu.RLock()
	defer mu.RUnlock()
	return db
}

func TestDatabaseConnection() error {
	mu.RLock()
	defer mu.RUnlock()

	if db == nil {
		return fmt.Errorf("conexão com o banco não foi inicializada")
	}
	return db.Ping()
}

// Nova função para reconexão
func ReconnectIfNeeded() error {
	if err := TestDatabaseConnection(); err != nil {
		debugLog("Tentando reconectar ao banco de dados...")
		CloseDatabase()
		return InitDatabase()
	}
	return nil
}
