// cmd/service/service.go
package main

import (
	"context"
	"log"
	"olmeda-service/internal/service"
	"olmeda-service/internal/websocket"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/sys/windows/svc"
)

type olmedaService struct {
	stopChan     chan bool
	wg           sync.WaitGroup
	wsClient     *websocket.WSClient
	isRunning    atomic.Bool
	monitor      *service.Monitor
	retryManager *websocket.RetryManager
}

var cmdsAccepted = svc.AcceptStop | svc.AcceptShutdown

func (m *olmedaService) Execute(args []string, r <-chan svc.ChangeRequest, changes chan<- svc.Status) (ssec bool, errno uint32) {
	debugLog("=== Execute: Iniciando execução do serviço ===")
	//debugLog("Alterando estado para: StartPending")
	//changes <- svc.Status{State: svc.StartPending}

	// Inicializa o banco de dados
	debugLog("Tentando inicializar banco de dados...")
	dsn := configuration.Database.DSN
	if !strings.Contains(dsn, "Driver=") {
		dsn = "Driver={Microsoft Access Driver (*.mdb, *.accdb)};" + dsn
	}
	if err := InitDatabase(); err != nil {
		debugLog("ERRO CRÍTICO: Falha ao inicializar banco de dados: %v", err)
		changes <- svc.Status{State: svc.Stopped}
		return true, 1
	}
	debugLog("Banco de dados inicializado com sucesso!")

	// Inicializa o WebSocket se estiver habilitado
	if configuration.WebSocket.Enabled {
		debugLog("Iniciando conexão WebSocket...")
		wsConfig := websocket.ClientConfig{
			ServerURL:      configuration.WebSocket.ServerURL,
			AuthToken:      configuration.WebSocket.AuthToken,
			ReconnectTimer: time.Duration(configuration.WebSocket.ReconnectInterval) * time.Second,
			PingInterval:   time.Duration(configuration.WebSocket.PingInterval) * time.Second,
			MessageHandler: &websocket.DefaultMessageHandler{
				RetryManager: m.retryManager,
			},
			ClinicID:   configuration.WebSocket.ClinicID,
			APIKey:     configuration.WebSocket.APIKey,
			InstanceID: configuration.WebSocket.InstanceID,
			UseSSL:     configuration.WebSocket.UseSSL,
		}

		// Cria o cliente WebSocket
		m.wsClient = websocket.NewWSClient(wsConfig)

		// Inicializa o Monitor com a mesma configuração
		debugLog("Iniciando monitor do sistema...")
		monitor, err := service.NewMonitor(dsn, wsConfig)
		debugLog("Usando DSN: %s", dsn)
		if err != nil {
			debugLog("ERRO CRÍTICO: Falha ao criar monitor: %v", err)
			changes <- svc.Status{State: svc.Stopped}
			return true, 1
		}
		m.monitor = monitor

		// Tenta conectar o WebSocket
		if err := m.wsClient.Connect(); err != nil {
			debugLog("Erro ao conectar WebSocket: %v", err)
		} else {
			debugLog("Conexão WebSocket estabelecida com sucesso")
		}
	} else {
		debugLog("WebSocket desabilitado nas configurações")
	}

	debugLog("Criando canal de parada...")
	m.stopChan = make(chan bool)

	// Criar contexto para o monitor
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel() // Garante que o contexto será cancelado quando a função retornar

	// Configura o estado como Running
	debugLog("Alterando estado para: Running")
	changes <- svc.Status{State: svc.Running, Accepts: cmdsAccepted}
	debugLog("Execute: Serviço está agora em execução")

	// Inicia monitoramento
	debugLog("Iniciando goroutines de monitoramento...")
	m.wg.Add(2)
	go m.runMonitoring()
	go m.monitor.Start(ctx) // Passa o contexto para o Start
	debugLog("Goroutines de monitoramento iniciadas")

	// Loop principal simplificado
	select {
	case c := <-r:
		switch c.Cmd {
		case svc.Stop, svc.Shutdown:
			debugLog("Recebido comando de parada via serviço")
			m.shutdown(changes)
			return false, 0
		}
	case <-m.stopChan:
		debugLog("Recebido sinal de parada via canal")
		m.shutdown(changes)
		return false, 0
	}

	// Processo de shutdown
	debugLog("Iniciando processo de shutdown...")

	// Fecha o WebSocket se estiver conectado
	if m.wsClient != nil {
		debugLog("Fechando conexão WebSocket...")
		m.wsClient.Close()
	}

	debugLog("Alterando estado para: StopPending")
	changes <- svc.Status{State: svc.StopPending}

	debugLog("Aguardando término das goroutines...")
	m.wg.Wait()
	debugLog("Todas as goroutines finalizadas")

	return false, 0
}

// Novo método para centralizar a lógica de shutdown
func (m *olmedaService) shutdown(changes chan<- svc.Status) {
	debugLog("Iniciando processo de shutdown...")
	m.isRunning.Store(false)

	// Limpa o RetryManager
	if m.retryManager != nil {
		debugLog("Finalizando RetryManager...")
		m.retryManager.Close()
	}

	// Para o monitor se estiver ativo
	if m.monitor != nil {
		debugLog("Parando monitor...")
		m.monitor.Stop()
	}

	// Fecha o WebSocket se estiver conectado
	if m.wsClient != nil {
		debugLog("Fechando conexão WebSocket...")
		m.wsClient.Close()
	}

	debugLog("Fechando canal de parada...")
	close(m.stopChan)

	debugLog("Alterando estado para: StopPending")
	changes <- svc.Status{State: svc.StopPending}

	debugLog("Aguardando término das goroutines...")
	m.wg.Wait()
	debugLog("Todas as goroutines finalizadas")
}

func (m *olmedaService) runMonitoring() {
	debugLog("=== Iniciando rotina de monitoramento ===")
	defer m.wg.Done()
	defer debugLog("=== Finalizando rotina de monitoramento ===")

	debugLog("Configurando ticker para 30 segundos")
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for m.isRunning.Load() {
		select {
		case <-m.stopChan:
			debugLog("Sinal de parada recebido no monitoramento")
			return
		case <-ticker.C:
			if !m.isRunning.Load() {
				return
			}
			debugLog("Executando verificação periódica...")

			// Verifica status do monitor
			if m.monitor != nil {
				metrics := m.monitor.GetMetrics()
				debugLog("Métricas do monitor - Registros: %d, Msgs: %d, Erros: %d",
					metrics.RecordsProcessed,
					metrics.MessagesSent,
					metrics.Errors)
			}

			// Verifica status do RetryManager
			if m.retryManager != nil {
				stats := m.retryManager.GetStats()
				debugLog("RetryManager - Pendentes: %d, Processadas: %d, Falhas: %d",
					stats.PendingMessages,
					stats.ProcessedMessages,
					stats.FailedMessages)
			}

			// Verifica conexão do banco
			if err := ReconnectIfNeeded(); err != nil {
				log.Printf("ERRO: Falha ao reconectar com o banco: %v", err)
			} else {
				debugLog("Verificação de conexão do banco concluída")
			}

			// Verifica conexão do WebSocket
			if m.wsClient != nil && !m.wsClient.IsConnected() {
				debugLog("WebSocket desconectado, tentando reconectar...")
				if err := m.wsClient.Connect(); err != nil {
					debugLog("Erro ao reconectar WebSocket: %v", err)
				} else {
					debugLog("WebSocket reconectado com sucesso")
				}
			}

			if configuration.Development.Enabled && configuration.Development.AutoRestart {
				debugLog("Modo desenvolvimento ativo - Verificando necessidade de auto-restart")
				// Implementar lógica de auto-restart se necessário
			}
		}
	}
}
