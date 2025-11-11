// internal/websocket/handlers.go
package websocket

import (
	"encoding/json"
	"fmt"
	"log"
	"time"
)

// Estruturas específicas para cada tipo de mensagem
type AgendamentoPayload struct {
	ID         string    `json:"id"`
	DataHora   time.Time `json:"dataHora"`
	DentistaID string    `json:"dentistaId"`
	PacienteID string    `json:"pacienteId"`
	Status     string    `json:"status"`
}

type NotificacaoPayload struct {
	Titulo   string `json:"titulo"`
	Mensagem string `json:"mensagem"`
	Nivel    string `json:"nivel"` // info, warning, error
}

type SincronizacaoPayload struct {
	Tabela   string          `json:"tabela"`
	Operacao string          `json:"operacao"` // insert, update, delete
	Dados    json.RawMessage `json:"dados"`
}

// Handlers concretos
type AgendamentoHandler struct {
	logger *log.Logger
}

type NotificacaoHandler struct {
	logger *log.Logger
}

type SincronizacaoHandler struct {
	logger *log.Logger
}

// Implementações do HandleMessage para cada handler
func (h *AgendamentoHandler) HandleMessage(msg *WebSocketMessage) error {
	var payload AgendamentoPayload
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		return fmt.Errorf("erro ao decodificar payload do agendamento: %v", err)
	}

	h.logger.Printf("Agendamento recebido: ID=%s, Data=%v", payload.ID, payload.DataHora)
	// Implementar lógica específica de agendamento aqui
	return nil
}

func (h *NotificacaoHandler) HandleMessage(msg *WebSocketMessage) error {
	var payload NotificacaoPayload
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		return fmt.Errorf("erro ao decodificar payload da notificação: %v", err)
	}

	h.logger.Printf("Notificação recebida: %s - %s [%s]", payload.Titulo, payload.Mensagem, payload.Nivel)
	// Implementar lógica específica de notificação aqui
	return nil
}

func (h *SincronizacaoHandler) HandleMessage(msg *WebSocketMessage) error {
	var payload SincronizacaoPayload
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		return fmt.Errorf("erro ao decodificar payload da sincronização: %v", err)
	}

	h.logger.Printf("Sincronização recebida: Tabela=%s, Operação=%s", payload.Tabela, payload.Operacao)
	// Implementar lógica específica de sincronização aqui
	return nil
}

// NewAgendamentoHandler cria um novo handler de agendamento
func NewAgendamentoHandler(logger *log.Logger) *AgendamentoHandler {
	return &AgendamentoHandler{logger: logger}
}

// NewNotificacaoHandler cria um novo handler de notificação
func NewNotificacaoHandler(logger *log.Logger) *NotificacaoHandler {
	return &NotificacaoHandler{logger: logger}
}

// NewSincronizacaoHandler cria um novo handler de sincronização
func NewSincronizacaoHandler(logger *log.Logger) *SincronizacaoHandler {
	return &SincronizacaoHandler{logger: logger}
}

// RegisterDefaultHandlers registra os handlers padrão para os tipos de mensagens básicos
func RegisterDefaultHandlers(logger *log.Logger) {
	// Criar instâncias dos handlers
	agendamentoHandler := NewAgendamentoHandler(logger)
	notificacaoHandler := NewNotificacaoHandler(logger)
	sincronizacaoHandler := NewSincronizacaoHandler(logger)

	// Registrar os handlers para cada tipo de mensagem
	RegisterHandler(TypeAgendamento, agendamentoHandler)
	RegisterHandler(TypeNotificacao, notificacaoHandler)
	RegisterHandler(TypeSincronizacao, sincronizacaoHandler)

	logger.Printf("Handlers padrão registrados com sucesso")
}
