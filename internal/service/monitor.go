// internal/service/monitor.go
package service

import (
	"context"
	"fmt"
	"log"
	"math"
	"math/rand"
	"olmeda-service/internal/database"
	"olmeda-service/internal/models"
	"olmeda-service/internal/websocket"
	"strings"
	"sync"
	"time"
)

type Monitor struct {
	db            *database.AccessDB
	ws            *websocket.WSClient
	done          chan bool
	checkInterval time.Duration
	isRunning     bool
	mu            sync.Mutex
	metrics       *Metrics
}

// Novo tipo para métricas
type Metrics struct {
	mu               sync.Mutex
	recordsProcessed int64
	messagesSent     int64
	messagesReceived int64
	errors           int64
	lastError        time.Time
}

var rng = rand.New(rand.NewSource(time.Now().UnixNano()))

// Primeiro, vamos definir constantes para os status
const (
	StatusEnviado    = "ENVIADO"
	StatusConfirmado = "CONFIRMADO"
	StatusErro       = "ERRO"
)

func NewMonitor(dbPath string, wsConfig websocket.ClientConfig) (*Monitor, error) {

	// Inicializar conexão com banco
	db, err := database.NewAccessDB(dbPath)
	if err != nil {
		return nil, err
	}

	// Criar cliente WebSocket
	ws := websocket.NewWSClient(wsConfig)

	return &Monitor{
		db:            db,
		ws:            ws,
		done:          make(chan bool),
		checkInterval: 30 * time.Second,
		metrics:       &Metrics{},
	}, nil
}

func (m *Monitor) Start(ctx context.Context) error {
	m.mu.Lock()
	if m.isRunning {
		m.mu.Unlock()
		return fmt.Errorf("monitor já está em execução")
	}
	m.isRunning = true
	m.mu.Unlock()

	// Conectar ao WebSocket com retry
	if err := m.connectWithRetry(ctx); err != nil {
		return err
	}

	ticker := time.NewTicker(m.checkInterval)
	defer ticker.Stop()

	// Iniciar heartbeat em uma goroutine separada
	go m.startHeartbeat(ctx)

	for {
		select {
		case <-ctx.Done():
			m.Stop()
			return nil
		case <-ticker.C:
			if err := m.processRecords(); err != nil {
				m.recordError(fmt.Sprintf("Erro ao processar registros: %v", err))
			}
		case msg := <-m.ws.Receive():
			if err := m.handleIncomingMessage(msg); err != nil {
				m.recordError(fmt.Sprintf("Erro ao processar mensagem recebida: %v", err))
			}
		}
	}
}

// Novo método para conexão com retry
func (m *Monitor) connectWithRetry(ctx context.Context) error {
	maxAttempts := 5
	initialDelay := time.Second
	maxDelay := time.Minute
	factor := 2.0

	for attempt := 0; attempt < maxAttempts; attempt++ {
		if err := m.ws.Connect(); err == nil {
			return nil
		}

		delay := time.Duration(float64(initialDelay) * math.Pow(factor, float64(attempt)))
		if delay > maxDelay {
			delay = maxDelay
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
			continue
		}
	}

	return fmt.Errorf("falha ao conectar após %d tentativas", maxAttempts)
}

// Novo método para heartbeat
func (m *Monitor) startHeartbeat(ctx context.Context) {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			msg := models.Message{
				ID:        generateMessageID(),
				Type:      "heartbeat",
				Timestamp: time.Now(),
			}
			if err := m.ws.SendMessage(msg); err != nil {
				m.recordError(fmt.Sprintf("Erro no heartbeat: %v", err))
			}
		}
	}
}

// Método para registrar erros nas métricas
func (m *Monitor) recordError(errMsg string) {
	m.metrics.mu.Lock()
	defer m.metrics.mu.Unlock()

	m.metrics.errors++
	m.metrics.lastError = time.Now()
	log.Print(errMsg)
}

type MetricsSnapshot struct {
	RecordsProcessed int64
	MessagesSent     int64
	MessagesReceived int64
	Errors           int64
	LastError        time.Time
}

func (m *Monitor) GetMetrics() MetricsSnapshot {
	m.metrics.mu.Lock()
	defer m.metrics.mu.Unlock()

	return MetricsSnapshot{
		RecordsProcessed: m.metrics.recordsProcessed,
		MessagesSent:     m.metrics.messagesSent,
		MessagesReceived: m.metrics.messagesReceived,
		Errors:           m.metrics.errors,
		LastError:        m.metrics.lastError,
	}
}

func (m *Monitor) Stop() {
	m.mu.Lock()
	if !m.isRunning {
		m.mu.Unlock()
		return
	}
	m.isRunning = false
	m.mu.Unlock()

	close(m.done)
	m.ws.Close()
	m.db.Close()
}

func (m *Monitor) processRecords() error {
	records, err := m.db.GetPendingRecords()
	if err != nil {
		return fmt.Errorf("erro ao buscar registros pendentes: %v", err)
	}

	for _, record := range records {
		// Aqui record é do tipo database.Record, então podemos acessar seus campos diretamente
		recordData := map[string]interface{}{
			"id":           record.ID,
			"data_criacao": record.DataCriacao,
			"status":       record.Status,
			"dados":        record.Dados,
		}

		msg := models.Message{
			ID:        generateMessageID(),
			Type:      models.TypeNewRecord,
			Timestamp: time.Now(),
			Payload: models.Payload{
				TableName: "Registros",
				Action:    "INSERT",
				Data:      recordData,
			},
		}

		if err := m.ws.SendMessage(msg); err != nil {
			log.Printf("Erro ao enviar mensagem: %v", err)
			continue
		}

		// Atualiza o status do registro para "ENVIADO"
		if err := m.db.UpdateRecordStatus(int64(record.ID), false, StatusEnviado); err != nil {
			log.Printf("Erro ao atualizar status do registro %d: %v", record.ID, err)
		}
	}

	return nil
}

// Adicione este método para processar mensagens recebidas
func (m *Monitor) handleIncomingMessage(msg models.Message) error {
	switch msg.Type {
	case models.TypeAck:
		// Processa confirmação de recebimento
		if payload, ok := msg.Payload.Data["record_id"].(float64); ok {
			recordID := int64(payload)
			return m.db.UpdateRecordStatus(recordID, true, StatusConfirmado)
		}
	case models.TypeError:
		// Processa mensagens de erro
		if payload, ok := msg.Payload.Data["record_id"].(float64); ok {
			recordID := int64(payload)
			return m.db.UpdateRecordStatus(recordID, false, StatusErro)
		}
	}
	return nil
}

// Atualize a função randomString para usar o rng local
func randomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	result := strings.Builder{}
	result.Grow(length)
	for i := 0; i < length; i++ {
		result.WriteByte(charset[rng.Intn(len(charset))])
	}
	return result.String()
}

func generateMessageID() string {
	return fmt.Sprintf("%d-%s", time.Now().UnixNano(), randomString(8))
}
