package main

import (
	"crypto/tls"
	"flag"
	"log"
	"net/http"
	"os"
	"time"

	socketio "github.com/googollee/go-socket.io"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // Permissivo para testes
	},
}

func handleWebSocket(w http.ResponseWriter, r *http.Request) {
	// Verificar headers de autenticação
	clinicID := r.Header.Get("X-Clinic-ID")
	apiKey := r.Header.Get("X-API-Key")
	instanceID := r.Header.Get("X-Instance-ID")

	if clinicID == "" || apiKey == "" || instanceID == "" {
		http.Error(w, "Credenciais ausentes", http.StatusUnauthorized)
		return
	}

	// Log das informações de conexão
	log.Printf("Nova conexão - Clinic: %s, Instance: %s", clinicID, instanceID)

	// Upgrade da conexão HTTP para WebSocket
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Erro no upgrade: %v", err)
		return
	}
	defer conn.Close()

	// Configurar ping/pong
	conn.SetPingHandler(func(string) error {
		return conn.WriteControl(websocket.PongMessage, []byte{}, time.Now().Add(time.Second))
	})

	// Loop principal
	for {
		messageType, message, err := conn.ReadMessage()
		if err != nil {
			log.Printf("Erro na leitura: %v", err)
			break
		}

		// Log da mensagem recebida
		log.Printf("Recebido: %s", string(message))

		// Echo da mensagem (para teste)
		if err := conn.WriteMessage(messageType, message); err != nil {
			log.Printf("Erro no envio: %v", err)
			break
		}
	}
}

func main() {
	// Manter as flags existentes
	port := flag.String("port", "8443", "porta do servidor")
	certFile := flag.String("cert", "certs/server.pem", "caminho do certificado SSL")
	keyFile := flag.String("key", "certs/server.key", "caminho da chave privada SSL")
	flag.Parse()

	// Configurar logger
	logger := log.New(os.Stdout, "[WSS-SERVER] ", log.LstdFlags)

	// Configurar Socket.IO
	server := socketio.NewServer(nil)

	server.OnConnect("/", func(s socketio.Conn) error {
		s.SetContext("")
		logger.Printf("Cliente conectado: %s", s.ID())
		return nil
	})

	server.OnEvent("/", "message", func(s socketio.Conn, msg string) {
		logger.Printf("Mensagem recebida: %s", msg)
	})

	server.OnError("/", func(s socketio.Conn, e error) {
		logger.Printf("Erro: %v", e)
	})

	server.OnDisconnect("/", func(s socketio.Conn, reason string) {
		logger.Printf("Cliente desconectado: %s - Razão: %s", s.ID(), reason)
	})

	go server.Serve()
	defer server.Close()

	// Configurar TLS
	cert, err := tls.LoadX509KeyPair(*certFile, *keyFile)
	if err != nil {
		logger.Fatalf("Erro ao carregar certificados: %v", err)
	}

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
	}

	// Configurar rotas
	http.Handle("/socket.io/", server)
	http.HandleFunc("/ws", handleWebSocket) // Mantendo a rota WebSocket original

	// Configurar servidor
	httpServer := &http.Server{
		Addr:      ":" + *port,
		TLSConfig: tlsConfig,
	}

	// Iniciar servidor
	logger.Printf("Servidor iniciado na porta %s", *port)
	if err := httpServer.ListenAndServeTLS("", ""); err != nil {
		logger.Fatalf("Erro ao iniciar servidor: %v", err)
	}
}
