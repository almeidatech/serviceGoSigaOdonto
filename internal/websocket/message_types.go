// internal/websocket/message_types.go

package websocket

import (
	"encoding/json"
	"fmt"
	"time"
)

// RetryPolicy define a política de retry para um tipo de mensagem
type RetryPolicy struct {
	MaxRetries     int           `json:"maxRetries"`     // Número máximo de tentativas
	InitialBackoff time.Duration `json:"initialBackoff"` // Tempo inicial entre tentativas
	MaxBackoff     time.Duration `json:"maxBackoff"`     // Tempo máximo entre tentativas
	BackoffFactor  float64       `json:"backoffFactor"`  // Fator de multiplicação para backoff exponencial
}

// RetryStatus mantém o estado das tentativas de processamento
type RetryStatus struct {
	Attempts    int       `json:"attempts"`    // Número de tentativas realizadas
	LastAttempt time.Time `json:"lastAttempt"` // Timestamp da última tentativa
	NextAttempt time.Time `json:"nextAttempt"` // Timestamp da próxima tentativa
	LastError   string    `json:"lastError"`   // Último erro ocorrido
	Status      string    `json:"status"`      // Status atual (pending, processing, failed, completed)
}

// Adicionar estrutura do ACK
type AckPayload struct {
	MessageID string    `json:"messageId"` // ID da mensagem original
	Status    string    `json:"status"`    // "received", "processed", "failed"
	Timestamp time.Time `json:"timestamp"`
	Error     string    `json:"error,omitempty"` // Opcional, apenas se status for "failed"
}

// Definição do WebSocketMessage primeiro
type WebSocketMessage struct {
	Type        MessageType     `json:"type"`
	ID          string          `json:"id"`
	Time        int64           `json:"time"`
	Payload     json.RawMessage `json:"payload"`
	Timeout     time.Duration   `json:"timeout,omitempty"`
	RetryPolicy *RetryPolicy    `json:"retryPolicy,omitempty"` // Política de retry específica para a mensagem
	RetryStatus *RetryStatus    `json:"retryStatus,omitempty"` // Status atual das tentativas
	AckRequired bool            `json:"ackRequired,omitempty"` // Nova propriedade
	AckReceived bool            `json:"ackReceived,omitempty"` // Nova propriedade
}

// Agora podemos definir a interface que usa WebSocketMessage
type MessageHandler interface {
	HandleMessage(msg *WebSocketMessage) error
}

type MessageType string

const (
	TypeAgendamento   MessageType = "agendamento"
	TypeNotificacao   MessageType = "notificacao"
	TypeSincronizacao MessageType = "sincronizacao"
	TypeAck           MessageType = "ack"
	StatusPending                 = "pending"
	StatusProcessing              = "processing"
	StatusFailed                  = "failed"
	StatusCompleted               = "completed"
	TypeMessage       MessageType = "message" // Mensagem normal
	TypeError         MessageType = "error"   // Mensagem de erro
	TypeRetry         MessageType = "retry"   // Mensagem de retry
)

// Map de handlers
var messageHandlers = make(map[MessageType]MessageHandler)

func RegisterHandler(msgType MessageType, handler MessageHandler) {
	messageHandlers[msgType] = handler
}

func GetMessageHandler(msgType MessageType) (MessageHandler, error) {
	handler, exists := messageHandlers[msgType]
	if !exists {
		return nil, fmt.Errorf("handler não encontrado para o tipo de mensagem: %v", msgType)
	}
	return handler, nil
}

// DefaultRetryPolicy retorna uma política de retry padrão
func DefaultRetryPolicy() *RetryPolicy {
	return &RetryPolicy{
		MaxRetries:     3,
		InitialBackoff: 1 * time.Second,
		MaxBackoff:     1 * time.Minute,
		BackoffFactor:  2.0,
	}
}

// NewRetryStatus cria um novo status de retry
func NewRetryStatus() *RetryStatus {
	return &RetryStatus{
		Attempts:    0,
		LastAttempt: time.Time{},
		NextAttempt: time.Now(),
		Status:      StatusPending,
	}
}

// ShouldRetry verifica se a mensagem deve ser retentada
func (msg *WebSocketMessage) ShouldRetry() bool {
	if msg.RetryStatus == nil || msg.RetryPolicy == nil {
		return false
	}

	return msg.RetryStatus.Attempts < msg.RetryPolicy.MaxRetries &&
		msg.RetryStatus.Status == StatusFailed &&
		time.Now().After(msg.RetryStatus.NextAttempt)
}

// UpdateRetryStatus atualiza o status de retry após uma tentativa
func (msg *WebSocketMessage) UpdateRetryStatus(err error) {
	if msg.RetryStatus == nil {
		msg.RetryStatus = NewRetryStatus()
	}

	msg.RetryStatus.Attempts++
	msg.RetryStatus.LastAttempt = time.Now()

	if err != nil {
		msg.RetryStatus.LastError = err.Error()
		msg.RetryStatus.Status = StatusFailed

		// Calcula próximo backoff
		backoff := msg.RetryPolicy.InitialBackoff *
			time.Duration(float64(msg.RetryStatus.Attempts)*msg.RetryPolicy.BackoffFactor)

		if backoff > msg.RetryPolicy.MaxBackoff {
			backoff = msg.RetryPolicy.MaxBackoff
		}

		msg.RetryStatus.NextAttempt = time.Now().Add(backoff)
	} else {
		msg.RetryStatus.Status = StatusCompleted
		msg.RetryStatus.LastError = ""
	}
}
