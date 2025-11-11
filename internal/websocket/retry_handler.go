// internal/websocket/retry_handler.go

package websocket

import (
	"log"
	"sync"
	"time"
)

// RetryManager gerencia o reprocessamento de mensagens
type RetryManager struct {
	messages        map[string]*WebSocketMessage // Mensagens aguardando retry
	mu              sync.RWMutex                 // Mutex para acesso concorrente
	logger          *log.Logger                  // Logger para registro de eventos
	processQueue    chan *WebSocketMessage       // Canal para processamento
	done            chan struct{}                // Canal para controle de shutdown
	pendingMessages map[string]*WebSocketMessage
	processedCount  int
	failedCount     int
}

type RetryStats struct {
	PendingMessages   int
	ProcessedMessages int
	FailedMessages    int
}

// NewRetryManager cria uma nova instância do RetryManager
func NewRetryManager(logger *log.Logger) *RetryManager {
	rm := &RetryManager{
		messages:       make(map[string]*WebSocketMessage),
		logger:         logger,
		processQueue:   make(chan *WebSocketMessage, 100),
		done:           make(chan struct{}),
		processedCount: 0,
		failedCount:    0,
	}

	// Inicia a goroutine de processamento
	go rm.processLoop()
	// Inicia a goroutine de verificação de retries
	go rm.retryLoop()

	return rm
}

// AddMessage adiciona uma mensagem para retry
func (rm *RetryManager) AddMessage(msg *WebSocketMessage) {
	if msg.ID == "" || msg.RetryPolicy == nil {
		return
	}

	rm.mu.Lock()
	defer rm.mu.Unlock()

	// Inicializa o status de retry se necessário
	if msg.RetryStatus == nil {
		msg.RetryStatus = NewRetryStatus()
	}

	rm.messages[msg.ID] = msg
	rm.processQueue <- msg
}

// processLoop processa mensagens da fila
func (rm *RetryManager) processLoop() {
	for {
		select {
		case <-rm.done:
			return
		case msg := <-rm.processQueue:
			rm.processMessage(msg)
		}
	}
}

// processMessage processa uma única mensagem
func (rm *RetryManager) processMessage(msg *WebSocketMessage) {
	// Atualiza status para processing
	msg.RetryStatus.Status = StatusProcessing

	// Tenta processar a mensagem
	err := ProcessMessage(msg.Payload)
	msg.UpdateRetryStatus(err)

	if err != nil {
		rm.logger.Printf("Erro ao processar mensagem %s (tentativa %d): %v",
			msg.ID, msg.RetryStatus.Attempts, err)
	} else {
		rm.logger.Printf("Mensagem %s processada com sucesso após %d tentativas",
			msg.ID, msg.RetryStatus.Attempts)

		// Remove mensagem processada com sucesso
		rm.mu.Lock()
		delete(rm.messages, msg.ID)
		rm.mu.Unlock()
	}
}

// retryLoop verifica periodicamente mensagens para retry
func (rm *RetryManager) retryLoop() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-rm.done:
			return
		case <-ticker.C:
			rm.checkRetries()
		}
	}
}

// checkRetries verifica mensagens que precisam ser retentadas
func (rm *RetryManager) checkRetries() {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	now := time.Now()
	for _, msg := range rm.messages {
		if msg.ShouldRetry() && now.After(msg.RetryStatus.NextAttempt) {
			select {
			case rm.processQueue <- msg:
				rm.logger.Printf("Agendando retry para mensagem %s (tentativa %d)",
					msg.ID, msg.RetryStatus.Attempts+1)
			default:
				rm.logger.Printf("Fila de processamento cheia, retry adiado para mensagem %s",
					msg.ID)
			}
		}
	}
}

// GetMessageStatus retorna o status atual de uma mensagem
func (rm *RetryManager) GetMessageStatus(msgID string) *RetryStatus {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	if msg, exists := rm.messages[msgID]; exists {
		return msg.RetryStatus
	}
	return nil
}

// Shutdown encerra o RetryManager de forma graciosa
func (rm *RetryManager) Shutdown() {
	close(rm.done)

	// Espera processamento pendente finalizar
	rm.mu.Lock()
	defer rm.mu.Unlock()

	for id, msg := range rm.messages {
		if msg.RetryStatus.Status == StatusProcessing {
			rm.logger.Printf("Mensagem %s interrompida durante shutdown", id)
		}
	}
}

func (rm *RetryManager) Close() {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	// Limpa as mensagens pendentes
	rm.pendingMessages = make(map[string]*WebSocketMessage)
	rm.logger.Println("RetryManager: Fechado e limpo")
}

func (rm *RetryManager) GetStats() RetryStats {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	return RetryStats{
		PendingMessages:   len(rm.pendingMessages),
		ProcessedMessages: rm.processedCount,
		FailedMessages:    rm.failedCount,
	}
}
