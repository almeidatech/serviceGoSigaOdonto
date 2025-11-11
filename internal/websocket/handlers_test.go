// internal/websocket/handlers_test.go
package websocket

import (
	"bytes"
	"encoding/json"
	"log"
	"testing"
	"time"
)

// SlowHandler para testes de timeout
type SlowHandler struct {
	delay time.Duration
}

// Implementar a interface MessageHandler
func (h *SlowHandler) HandleMessage(msg *WebSocketMessage) error {
	time.Sleep(h.delay)
	return nil
}

func TestAgendamentoHandler_HandleMessage(t *testing.T) {
	// Configurar um buffer para capturar os logs
	var logBuffer bytes.Buffer
	logger := log.New(&logBuffer, "", 0)

	// Criar o handler
	handler := NewAgendamentoHandler(logger)

	// Criar uma mensagem de teste
	payload := AgendamentoPayload{
		ID:         "test123",
		DataHora:   time.Now(),
		DentistaID: "dent456",
		PacienteID: "pac789",
		Status:     "agendado",
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("Erro ao criar payload de teste: %v", err)
	}

	msg := &WebSocketMessage{
		Type:    TypeAgendamento,
		ID:      "msg123",
		Time:    time.Now().Unix(),
		Payload: payloadBytes,
	}

	// Testar o handler
	err = handler.HandleMessage(msg)
	if err != nil {
		t.Errorf("HandleMessage retornou erro: %v", err)
	}

	// Verificar se o log foi gerado
	logOutput := logBuffer.String()
	if !bytes.Contains(logBuffer.Bytes(), []byte("Agendamento recebido")) {
		t.Errorf("Log esperado não encontrado. Log gerado: %s", logOutput)
	}
}

func TestNotificacaoHandler_HandleMessage(t *testing.T) {
	var logBuffer bytes.Buffer
	logger := log.New(&logBuffer, "", 0)

	handler := NewNotificacaoHandler(logger)

	payload := NotificacaoPayload{
		Titulo:   "Teste",
		Mensagem: "Mensagem de teste",
		Nivel:    "info",
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("Erro ao criar payload de teste: %v", err)
	}

	msg := &WebSocketMessage{
		Type:    TypeNotificacao,
		ID:      "msg456",
		Time:    time.Now().Unix(),
		Payload: payloadBytes,
	}

	err = handler.HandleMessage(msg)
	if err != nil {
		t.Errorf("HandleMessage retornou erro: %v", err)
	}

	logOutput := logBuffer.String()
	if !bytes.Contains(logBuffer.Bytes(), []byte("Notificação recebida")) {
		t.Errorf("Log esperado não encontrado. Log gerado: %s", logOutput)
	}
}

// TestInvalidPayload verifica se os handlers tratam corretamente payloads inválidos
func TestInvalidPayload(t *testing.T) {
	logger := log.New(&bytes.Buffer{}, "", 0)

	tests := []struct {
		name    string
		handler MessageHandler
		msgType MessageType
		payload string
	}{
		{
			name:    "Agendamento Inválido",
			handler: NewAgendamentoHandler(logger),
			msgType: TypeAgendamento,
			payload: `{"invalid": "json"`,
		},
		{
			name:    "Notificação Inválida",
			handler: NewNotificacaoHandler(logger),
			msgType: TypeNotificacao,
			payload: `{"invalid": "json"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := &WebSocketMessage{
				Type:    tt.msgType,
				ID:      "test",
				Time:    time.Now().Unix(),
				Payload: json.RawMessage(tt.payload),
			}

			err := tt.handler.HandleMessage(msg)
			if err == nil {
				t.Error("Esperava erro com payload inválido, mas não recebeu nenhum")
			}
		})
	}
}

// TestProcessWithTimeout testa o comportamento do timeout no processamento de mensagens
func TestProcessWithTimeout(t *testing.T) {
	tests := []struct {
		name         string
		timeout      time.Duration
		handlerDelay time.Duration
		wantErr      bool
	}{
		{
			name:         "Deve completar dentro do timeout",
			timeout:      100 * time.Millisecond,
			handlerDelay: 50 * time.Millisecond,
			wantErr:      false,
		},
		{
			name:         "Deve falhar por timeout",
			timeout:      50 * time.Millisecond,
			handlerDelay: 100 * time.Millisecond,
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := &SlowHandler{delay: tt.handlerDelay}
			msg := &WebSocketMessage{
				Type:    "test",
				ID:      "timeout-test",
				Time:    time.Now().Unix(),
				Payload: []byte("test message"),
			}

			err := ProcessWithTimeout(msg, handler, tt.timeout)
			if (err != nil) != tt.wantErr {
				t.Errorf("ProcessWithTimeout() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
