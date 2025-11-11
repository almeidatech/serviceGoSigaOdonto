// internal/websocket/process.go

package websocket

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// ProcessMessageWithRetry é a nova função principal que usa o RetryManager
func ProcessMessageWithRetry(messageBytes []byte, retryManager *RetryManager) error {
	var msg WebSocketMessage
	if err := json.Unmarshal(messageBytes, &msg); err != nil {
		return fmt.Errorf("erro ao decodificar mensagem: %v", err)
	}

	// Se a mensagem não tem política de retry, processa normalmente
	if msg.RetryPolicy == nil {
		return ProcessMessage(messageBytes)
	}

	// Adiciona a mensagem ao retry manager
	retryManager.AddMessage(&msg)
	return nil
}

// ProcessMessage mantém a lógica original mas com algumas modificações para retry
func ProcessMessage(messageBytes []byte) error {
	var msg WebSocketMessage
	if err := json.Unmarshal(messageBytes, &msg); err != nil {
		return fmt.Errorf("erro ao decodificar mensagem: %v", err)
	}

	// Define o timeout padrão se não especificado
	timeout := 30 * time.Second
	if msg.Timeout > 0 {
		timeout = time.Duration(msg.Timeout) * time.Millisecond
	}

	// Cria um contexto com timeout
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// Canal para receber o resultado do processamento
	done := make(chan error)

	// Processa a mensagem em uma goroutine separada
	go func() {
		handler, err := GetMessageHandler(msg.Type)
		if err != nil {
			done <- fmt.Errorf("handler não encontrado: %v", err)
			return
		}

		// Processa a mensagem
		err = handler.HandleMessage(&msg)

		// Atualiza o status de retry se existir
		if msg.RetryStatus != nil {
			if err != nil {
				msg.RetryStatus.LastError = err.Error()
				msg.RetryStatus.Status = StatusFailed
			} else {
				msg.RetryStatus.Status = StatusCompleted
			}
			msg.RetryStatus.LastAttempt = time.Now()
		}

		done <- err
	}()

	// Espera pelo resultado ou timeout
	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		timeoutErr := fmt.Errorf("timeout ao processar mensagem ID %s: %v", msg.ID, ctx.Err())

		// Atualiza status em caso de timeout
		if msg.RetryStatus != nil {
			msg.RetryStatus.LastError = timeoutErr.Error()
			msg.RetryStatus.Status = StatusFailed
			msg.RetryStatus.LastAttempt = time.Now()
		}

		return timeoutErr
	}
}
