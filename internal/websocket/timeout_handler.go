// internal/websocket/timeout_handler.go

package websocket

import (
	"context"
	"fmt"
	"time"
)

// ProcessWithTimeout processa uma mensagem com um timeout definido
func ProcessWithTimeout(msg *WebSocketMessage, handler MessageHandler, defaultTimeout time.Duration) error {
	// Se não houver timeout definido na mensagem, usa o timeout passado como parâmetro
	timeout := defaultTimeout
	if msg.Timeout > 0 {
		timeout = msg.Timeout
	}

	// Criar um contexto com timeout
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// Canal para receber o resultado do processamento
	done := make(chan error)

	// Processar a mensagem em uma goroutine separada
	go func() {
		done <- handler.HandleMessage(msg)
	}()

	// Esperar pelo resultado ou timeout
	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		return fmt.Errorf("timeout ao processar mensagem ID %s: %v", msg.ID, ctx.Err())
	}
}
