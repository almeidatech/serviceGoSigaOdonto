// /internal/websocket/ack_handler.go

package websocket

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

type AckHandler struct {
	pendingAcks map[string]*WebSocketMessage
	ackChannels map[string]chan struct{} // Novo campo
	client      *WSClient
	timeout     time.Duration
	mu          sync.Mutex // Novo campo
}

func NewAckHandler(client *WSClient) *AckHandler {
	return &AckHandler{
		pendingAcks: make(map[string]*WebSocketMessage),
		ackChannels: make(map[string]chan struct{}), // Inicializa o novo map
		client:      client,
		timeout:     30 * time.Second,
	}
}

func (ah *AckHandler) RegisterAckChannel(messageID string, ch chan struct{}) {
	ah.mu.Lock()
	defer ah.mu.Unlock()
	ah.ackChannels[messageID] = ch
}

func (ah *AckHandler) UnregisterAckChannel(messageID string) {
	ah.mu.Lock()
	defer ah.mu.Unlock()
	delete(ah.ackChannels, messageID)
}

func (h *AckHandler) HandleMessage(msg *WebSocketMessage) error {
	var ackPayload AckPayload
	if err := json.Unmarshal(msg.Payload, &ackPayload); err != nil {
		return fmt.Errorf("erro ao decodificar ACK: %v", err)
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	// Notifica o canal de ACK se existir
	if ch, exists := h.ackChannels[ackPayload.MessageID]; exists {
		close(ch)
		delete(h.ackChannels, ackPayload.MessageID)
	}

	// Procura a mensagem original
	originalMsg, exists := h.pendingAcks[ackPayload.MessageID]
	if !exists {
		return fmt.Errorf("mensagem original não encontrada para ACK: %s", ackPayload.MessageID)
	}

	// Atualiza o status da mensagem original
	originalMsg.AckReceived = true
	delete(h.pendingAcks, ackPayload.MessageID)

	// Se o ACK indicar falha, podemos acionar o retry
	if ackPayload.Status == "failed" {
		originalMsg.RetryStatus.Status = StatusFailed
		originalMsg.RetryStatus.LastError = ackPayload.Error
		return h.client.handleRetry(originalMsg)
	}

	return nil
}

func (h *AckHandler) WaitForAck(msg *WebSocketMessage) {
	if !msg.AckRequired {
		return
	}

	h.pendingAcks[msg.ID] = msg

	// Inicia um timer para timeout do ACK
	go func() {
		timer := time.NewTimer(h.timeout)
		<-timer.C

		// Se ainda não recebeu ACK após o timeout
		if msg, exists := h.pendingAcks[msg.ID]; exists {
			msg.RetryStatus.Status = StatusFailed
			msg.RetryStatus.LastError = "ACK timeout"
			h.client.handleRetry(msg)
			delete(h.pendingAcks, msg.ID)
		}
	}()
}
