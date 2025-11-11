package main

import (
	"crypto/tls"
	"crypto/x509"
	"flag"
	"log"
	"os"
	"time"

	"github.com/gorilla/websocket"
)

func main() {
	// Configurar flags
	serverAddr := flag.String("addr", "localhost:8443", "endereço do servidor")
	certFile := flag.String("cert", "certs/ca.pem", "caminho do certificado CA")
	flag.Parse()

	// Configurar logger
	logger := log.New(os.Stdout, "[WSS-CLIENT] ", log.LstdFlags)

	// Carregar certificado CA
	caCert, err := os.ReadFile(*certFile)
	if err != nil {
		logger.Fatalf("Erro ao ler certificado CA: %v", err)
	}

	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(caCert)

	// Configurar TLS
	tlsConfig := &tls.Config{
		RootCAs:            caCertPool,
		InsecureSkipVerify: true, // Apenas para teste local!
	}

	// Configurar dialer do WebSocket
	dialer := websocket.Dialer{
		TLSClientConfig: tlsConfig,
		Proxy:           nil,
	}

	// Headers para autenticação
	headers := map[string][]string{
		"X-Clinic-ID":   {"clinic-test-123"},
		"X-API-Key":     {"api-key-test-456"},
		"X-Instance-ID": {"instance-test-789"},
	}

	// Conectar ao servidor
	url := "wss://" + *serverAddr + "/ws"
	logger.Printf("Conectando a %s", url)

	conn, _, err := dialer.Dial(url, headers)
	if err != nil {
		logger.Fatalf("Erro ao conectar: %v", err)
	}
	defer conn.Close()

	// Configurar handler para pong
	conn.SetPongHandler(func(string) error {
		logger.Printf("Pong recebido")
		return nil
	})

	// Enviar algumas mensagens de teste
	messages := []string{
		"Teste 1: Mensagem simples",
		"Teste 2: Outra mensagem",
		"Teste 3: Última mensagem",
	}

	for _, msg := range messages {
		logger.Printf("Enviando: %s", msg)
		err := conn.WriteMessage(websocket.TextMessage, []byte(msg))
		if err != nil {
			logger.Printf("Erro ao enviar mensagem: %v", err)
			return
		}

		// Aguardar e ler resposta
		_, response, err := conn.ReadMessage()
		if err != nil {
			logger.Printf("Erro ao ler resposta: %v", err)
			return
		}
		logger.Printf("Resposta recebida: %s", string(response))

		// Pequena pausa entre mensagens
		time.Sleep(1 * time.Second)
	}

	// Manter conexão por um tempo para testes
	logger.Printf("Testes concluídos. Mantendo conexão por 5 segundos...")
	time.Sleep(5 * time.Second)
}
