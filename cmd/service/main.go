// cmd/service/main.go
package main

import (
	"database/sql"
	"fmt"
	"log"
	"olmeda-service/internal/websocket"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	_ "github.com/alexbrainman/odbc"
	"golang.org/x/sys/windows/svc"
)

// Variáveis globais
var (
	configuration Config // Esta struct já está definida em config.go
	logFile       *os.File
)

func main() {

	// Força o stdout a ser mostrado imediatamente
	fmt.Print("\n=== TESTE DIRETO DE ARQUIVO ===\n")
	os.Stdout.Sync()

	dbPath := "D:\\teste\\odontoDB.mdb"
	if _, err := os.Stat(dbPath); err == nil {
		fmt.Printf("✅ Arquivo existe em: %s\n", dbPath)
		os.Stdout.Sync()

		// Tenta abrir o arquivo diretamente
		if file, err := os.OpenFile(dbPath, os.O_RDWR, 0666); err != nil {
			fmt.Printf("❌ Erro ao abrir arquivo: %v\n", err)
		} else {
			fmt.Println("✅ Arquivo aberto com sucesso!")
			file.Close()

			// Tenta listar o diretório pai
			if files, err := os.ReadDir("D:\\teste"); err != nil {
				fmt.Printf("❌ Erro ao ler diretório: %v\n", err)
			} else {
				fmt.Println("\nArquivos no diretório D:\\teste:")
				for _, file := range files {
					fmt.Printf("- %s\n", file.Name())
				}
			}
		}
	} else {
		fmt.Printf("❌ Arquivo não encontrado: %v\n", err)

		// Verifica se o diretório existe
		if _, err := os.Stat("D:\\teste"); err != nil {
			fmt.Printf("❌ Diretório D:\\teste não existe: %v\n", err)
		} else {
			fmt.Println("✅ Diretório D:\\teste existe")
		}
	}

	fmt.Println("=== VERIFICAÇÃO INICIAL DE DRIVERS ===")
	drivers := sql.Drivers()
	fmt.Printf("Drivers SQL disponíveis: %v\n", drivers)

	// Teste detalhado de conexão
	fmt.Println("\n=== TESTE DETALHADO DE CONEXÃO ===")

	// Teste 1: Conexão simples
	dsn1 := "Driver={Microsoft Access Driver (*.mdb, *.accdb)}"
	fmt.Printf("Testando DSN 1: %s\n", dsn1)
	db1, err := sql.Open("odbc", dsn1)
	if err != nil {
		fmt.Printf("❌ Teste 1 falhou na abertura: %v\n", err)
	} else {
		err = db1.Ping()
		if err != nil {
			fmt.Printf("❌ Teste 1 falhou no ping: %v\n", err)
		} else {
			fmt.Printf("✅ Teste 1 OK\n")
		}
		db1.Close()
	}

	// Teste 2: Conexão com caminho do banco
	dsn2 := fmt.Sprintf("Driver={Microsoft Access Driver (*.mdb, *.accdb)};DBQ=%s", "D:\\teste\\odontoDB.mdb")
	fmt.Printf("\nTestando DSN 2: %s\n", dsn2)
	db2, err := sql.Open("odbc", dsn2)
	if err != nil {
		fmt.Printf("❌ Teste 2 falhou na abertura: %v\n", err)
	} else {
		err = db2.Ping()
		if err != nil {
			fmt.Printf("❌ Teste 2 falhou no ping: %v\n", err)
		} else {
			fmt.Printf("✅ Teste 2 OK\n")
		}
		db2.Close()
	}

	fmt.Println("\n=== VERIFICAÇÃO DO ARQUIVO ===")
	if _, err := os.Stat("D:\\teste\\odontoDB.mdb"); err != nil {
		if os.IsNotExist(err) {
			fmt.Println("❌ Arquivo do banco NÃO existe no caminho especificado")
		} else {
			fmt.Printf("❌ Erro ao verificar arquivo: %v\n", err)
		}
	} else {
		fmt.Println("✅ Arquivo do banco existe no caminho especificado")
		if file, err := os.Open("D:\\teste\\odontoDB.mdb"); err != nil {
			fmt.Printf("❌ Sem permissão para abrir o arquivo: %v\n", err)
		} else {
			file.Close()
			fmt.Println("✅ Permissões de acesso ao arquivo OK!")
		}
	}

	fmt.Println("=====================================")

	// Primeiro, vamos garantir que podemos obter o caminho do executável
	exePath, err := os.Executable()
	if err != nil {
		fmt.Printf("ERRO CRÍTICO: Falha ao obter caminho do executável: %v\n", err)
		return
	}
	execDir := filepath.Dir(exePath)
	fmt.Printf("Diretório do executável: %s\n", execDir)

	// Carrega as configurações
	fmt.Printf("Carregando configurações...\n")
	if err := loadConfig(exePath); err != nil {
		fmt.Printf("ERRO CRÍTICO: Falha ao carregar configuração: %v\n", err)
		return
	}
	fmt.Printf("Configurações carregadas com sucesso\n")

	fmt.Printf("Tentando abrir arquivo de log em: %s\n", configuration.LogPath)
	fmt.Printf("Diretório atual: %s\n", execDir)

	// Configura o sistema de logs
	fmt.Printf("Configurando sistema de logs...\n")
	logFile, err = os.OpenFile(configuration.LogPath,
		os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Printf("ERRO CRÍTICO: Falha ao abrir arquivo de log (%s): %v\n",
			configuration.LogPath, err)
		return
	}

	log.SetOutput(logFile)
	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds)

	// A partir daqui, usamos debugLog para logging
	debugLog("================================================================")
	debugLog("================= INICIANDO OLMEDA SERVICE =======================")
	debugLog("================================================================")

	debugLog("Informações de Inicialização:")
	debugLog("- Caminho do executável: %s", exePath)
	debugLog("- Diretório de trabalho: %s", execDir)
	debugLog("- Arquivo de log: %s", configuration.LogPath)
	debugLog("- DSN do banco: %s", configuration.Database.DSN)
	debugLog("- Nível de log: %s", configuration.LogLevel)

	// Inicializa a conexão com o banco de dados
	debugLog("Inicializando conexão com o banco de dados...")
	if err := InitDatabase(); err != nil {
		debugLog("ERRO CRÍTICO: Falha ao conectar ao banco de dados: %v", err)
		log.Fatalf("Erro ao conectar ao banco de dados: %v", err)
	}
	defer CloseDatabase()
	debugLog("Conexão com o banco de dados estabelecida com sucesso")

	// Log das configurações do WebSocket
	if configuration.WebSocket.Enabled {
		debugLog("WebSocket está ATIVO:")
		debugLog("- Server URL: %s", configuration.WebSocket.ServerURL)
		debugLog("- Porta: %d", configuration.WebSocket.Port)
		debugLog("- Intervalo de Reconexão: %d segundos", configuration.WebSocket.ReconnectInterval)
		debugLog("- Intervalo de Ping: %d segundos", configuration.WebSocket.PingInterval)
	} else {
		debugLog("WebSocket está DESATIVADO")
	}

	if configuration.Development.Enabled {
		debugLog("MODO DESENVOLVIMENTO ATIVO:")
		debugLog("- Auto Restart: %v", configuration.Development.AutoRestart)
		debugLog("- Debug Log: %v", configuration.Development.DebugLog)
	}

	const svcName = "OlmedaService"

	// Verifica se está rodando como serviço
	debugLog("Verificando tipo de sessão...")
	isService, err := svc.IsWindowsService()
	if err != nil {
		debugLog("ERRO CRÍTICO: Falha ao determinar tipo de sessão: %v", err)
		log.Fatalf("Falha ao determinar tipo de sessão: %v", err)
	}
	debugLog("Tipo de sessão: %s", map[bool]string{true: "Serviço", false: "Interativa"}[isService])

	// Cria instância do serviço
	srv := &olmedaService{
		stopChan:     make(chan bool),
		wsClient:     nil,
		retryManager: websocket.NewRetryManager(log.New(logFile, "[RETRY] ", log.LstdFlags)),
	}

	if isService {
		debugLog("Iniciando em modo serviço Windows")
		debugLog("Nome do serviço: %s", svcName)
		err = svc.Run(svcName, srv)
		if err != nil {
			debugLog("ERRO CRÍTICO: Serviço falhou: %v", err)
			log.Printf("Serviço falhou: %v\n", err)
			return
		}
	} else {
		debugLog("Iniciando em modo console (desenvolvimento)")
		log.Printf("Iniciando %s em modo console...\n", svcName)

		// Configura canais para modo console
		changes := make(chan svc.Status)
		requests := make(chan svc.ChangeRequest)

		// Configura captura de sinais
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

		// Inicia o serviço em uma goroutine
		done := make(chan bool)
		go func() {
			srv.Execute([]string{}, requests, changes)
			close(done)
		}()

		debugLog("Serviço iniciado em modo console - aguardando Ctrl+C")
		log.Println("Pressione Ctrl+C para sair")

		// Aguarda por sinal de interrupção ou término do serviço
		select {
		case sig := <-sigChan:
			debugLog("Sinal recebido: %v", sig)
			log.Printf("Encerrando serviço...")
			srv.stopChan <- true
		case <-done:
			debugLog("Serviço encerrado naturalmente")
		}

		// Pequena pausa para garantir que os logs sejam escritos
		time.Sleep(time.Second)
		debugLog("Processo de encerramento concluído")
	}

	debugLog("================================================================")
	debugLog("================= FINALIZANDO OLMEDA SERVICE ====================")
	debugLog("================================================================")
}
