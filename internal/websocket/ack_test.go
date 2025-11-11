// /internal/websocket/ack_test.go

package websocket

import (
	"testing"
	"time"
)

func TestAckHandler_BasicFlow(t *testing.T) {
	client := &WSClient{} // Mock do cliente
	ackHandler := NewAckHandler(client)

	// Criar mensagem que requer ACK
	msg := &WebSocketMessage{
		ID:          "test-msg-1",
		Type:        TypeMessage,
		Time:        time.Now().Unix(),
		AckRequired: true,
	}

	// Registrar mensagem para ACK
	ackHandler.WaitForAck(msg)

	// Verificar se está pendente
	if _, exists := ackHandler.pendingAcks[msg.ID]; !exists {
		t.Error("Mensagem deveria estar pendente de ACK")
	}

	// Simular recebimento de ACK
	ackMsg := &WebSocketMessage{
		Type: TypeAck,
		Time: time.Now().Unix(),
		Payload: []byte(`{
            "messageId": "test-msg-1",
            "status": "success"
        }`),
	}

	err := ackHandler.HandleMessage(ackMsg)
	if err != nil {
		t.Errorf("Erro ao processar ACK: %v", err)
	}

	// Verificar se foi removida dos pendentes
	if _, exists := ackHandler.pendingAcks[msg.ID]; exists {
		t.Error("Mensagem não deveria estar mais pendente após ACK")
	}
}

func TestAckHandler_Timeout(t *testing.T) {
	client := &WSClient{} // Mock do cliente
	ackHandler := NewAckHandler(client)
	ackHandler.timeout = 100 * time.Millisecond // Reduzir timeout para o teste

	msg := &WebSocketMessage{
		ID:          "test-msg-2",
		Type:        TypeMessage,
		Time:        time.Now().Unix(),
		AckRequired: true,
		RetryStatus: &RetryStatus{}, // Inicializar RetryStatus
	}

	// Registrar mensagem
	ackHandler.WaitForAck(msg)

	// Esperar pelo timeout
	time.Sleep(150 * time.Millisecond)

	// Verificar se foi removida dos pendentes após timeout
	if _, exists := ackHandler.pendingAcks[msg.ID]; exists {
		t.Error("Mensagem deveria ter sido removida após timeout")
	}
}
