// internal/websocket/client.go
package websocket

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"math/rand"
	"net/http"
	"net/url"
	"olmeda-service/internal/models"
	"os"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// Constantes globais
const (
	defaultPingInterval = 25 * time.Second
	defaultPingTimeout  = 10 * time.Second
	socketIOVersion     = "4"
	socketIOTransport   = "websocket"
	defaultBufferSize   = 100
	maxMessageSize      = 512 * 1024 // 512KB
	engineIOVersion     = "4"
	defaultTimeout      = 45 * time.Second
	maxPayloadSize      = 1024 * 1024 // 1MB
)

type ConnectionState int

const (
	CONNECT       = "0"
	DISCONNECT    = "1"
	EVENT         = "2"
	ACK           = "3"
	CONNECT_ERROR = "4"
	BINARY_EVENT  = "5"
	BINARY_ACK    = "6"
)
const (
	writeWait  = 10 * time.Second
	pongWait   = 60 * time.Second
	pingPeriod = (pongWait * 9) / 10
)

const (
	StateDisconnected ConnectionState = iota
	StateConnecting
	StateConnected
	StateReconnecting
)

// WSClient representa nosso cliente WebSocket
type WSClient struct {
	serverURL       string
	authToken       string
	conn            *websocket.Conn
	mu              sync.Mutex
	done            chan struct{}
	state           ConnectionState
	reconnectTimer  time.Duration
	pingInterval    time.Duration
	messageHandler  MessageHandler
	reconnecting    bool
	logger          *log.Logger
	messageHandlers map[MessageType]MessageHandler
	ackHandler      *AckHandler
	apiKey          string // Novo
	clinicID        string // Novo
	instanceID      string // Novo
	useSSL          bool   // Novo
	send            chan Message
	sessionID       string

	// Novas configurações para backoff exponencial
	reconnectCount int
	maxRetries     int
	initialBackoff time.Duration
	maxBackoff     time.Duration
	lastError      error
	receiveChan    chan models.Message
	ackTimeout     time.Duration
	stats          WSStats
	statsMu        sync.RWMutex // Mutex específico para estatísticas
}

// Nova struct para handshake
type HandshakeResponse struct {
	Sid          string   `json:"sid"`
	Upgrades     []string `json:"upgrades"`
	PingInterval int      `json:"pingInterval"`
	PingTimeout  int      `json:"pingTimeout"`
}

// ClientConfig contém as configurações do cliente
type ClientConfig struct {
	ServerURL      string
	AuthToken      string
	APIKey         string // Novo
	ClinicID       string // Novo
	InstanceID     string // Novo
	UseSSL         bool   // Novo
	ReconnectTimer time.Duration
	PingInterval   time.Duration
	MessageHandler MessageHandler
	LogFile        string        // Novo: arquivo para logs
	MaxRetries     int           // Novo: número máximo de tentativas
	InitialBackoff time.Duration // Novo: tempo inicial entre tentativas
	MaxBackoff     time.Duration // Novo: tempo máximo entre tentativas
	Logger         *log.Logger
	AckTimeout     time.Duration
}

// Atualizar o defaultMessageHandler para implementar a interface
type DefaultMessageHandler struct {
	logger       *log.Logger
	RetryManager *RetryManager
}

type WSStats struct {
	MessagesReceived int64
	MessagesSent     int64
	Reconnections    int64
	LastReconnectAt  time.Time
	ConnectedSince   time.Time
	BytesSent        int64 // Estatística adicional útil
	BytesReceived    int64 // Estatística adicional útil
}

func (h *DefaultMessageHandler) HandleMessage(msg *WebSocketMessage) error {
	h.logger.Printf("Mensagem recebida: %s", string(msg.Payload))
	return nil
}

func maskSensitiveValue(value string) string {
	if len(value) <= 8 {
		return "****"
	}
	return value[:4] + "****" + value[len(value)-4:]
}

// NewWSClient cria uma nova instância do cliente
func NewWSClient(config ClientConfig) *WSClient {
	// Configurar valores padrão
	if config.ReconnectTimer == 0 {
		config.ReconnectTimer = 30 * time.Second
	}
	if config.PingInterval == 0 {
		config.PingInterval = 30 * time.Second
	}
	if config.MessageHandler == nil {
		config.MessageHandler = &DefaultMessageHandler{logger: config.Logger}
	}
	if config.MaxRetries == 0 {
		config.MaxRetries = 10
	}
	if config.InitialBackoff == 0 {
		config.InitialBackoff = time.Second
	}
	if config.MaxBackoff == 0 {
		config.MaxBackoff = time.Minute
	}

	// Configurar valor padrão para ackTimeout
	if config.AckTimeout == 0 {
		config.AckTimeout = 30 * time.Second // valor padrão de 30 segundos
	}

	// Configurar logger
	var logger *log.Logger
	if config.LogFile != "" {
		logFile, err := os.OpenFile(config.LogFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			log.Printf("Erro ao abrir arquivo de log: %v. Usando stdout", err)
			logger = log.New(os.Stdout, "[WebSocket] ", log.LstdFlags)
		} else {
			logger = log.New(logFile, "[WebSocket] ", log.LstdFlags)
		}
	} else {
		logger = log.New(os.Stdout, "[WebSocket] ", log.LstdFlags)
	}

	client := &WSClient{
		serverURL:       config.ServerURL,
		authToken:       config.AuthToken,
		apiKey:          config.APIKey,     // Novo
		clinicID:        config.ClinicID,   // Novo
		instanceID:      config.InstanceID, // Novo
		useSSL:          config.UseSSL,     // Novo
		done:            make(chan struct{}),
		state:           StateDisconnected,
		reconnectTimer:  config.ReconnectTimer,
		pingInterval:    config.PingInterval,
		messageHandler:  config.MessageHandler,
		logger:          logger,
		maxRetries:      config.MaxRetries,
		initialBackoff:  config.InitialBackoff,
		maxBackoff:      config.MaxBackoff,
		messageHandlers: make(map[MessageType]MessageHandler),
		ackTimeout:      config.AckTimeout,
	}

	// Inicializa o ackHandler após ter a instância do client
	client.ackHandler = NewAckHandler(client)

	// Registrar handlers padrão
	RegisterDefaultHandlers(logger)

	// Registra o handler de ACK
	client.RegisterMessageHandler(TypeAck, client.ackHandler)

	return client
}

// Estrutura para mensagens que serão enviadas
type Message struct {
	Type int
	Data []byte
}

// Canal para mensagens de saída
func (c *WSClient) initChannels() {
	c.send = make(chan Message, 256)
}

func (c *WSClient) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.mu.Lock()
		if c.conn != nil {
			c.conn.Close()
			c.conn = nil
		}
		c.state = StateDisconnected
		c.mu.Unlock()

		// Tenta reconectar se não estiver em shutdown
		if !c.isDone() {
			go c.reconnectWithBackoff()
		}
	}()

	for {
		select {
		case <-c.done:
			return

		case message, ok := <-c.send:
			c.mu.Lock()
			if !ok || c.conn == nil {
				c.mu.Unlock()
				return
			}

			err := c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err != nil {
				c.logger.Printf("Erro ao definir write deadline: %v", err)
				c.mu.Unlock()
				return
			}

			if err := c.conn.WriteMessage(message.Type, message.Data); err != nil {
				c.logger.Printf("Erro ao enviar mensagem: %v", err)
				c.mu.Unlock()
				return
			}
			c.mu.Unlock()

		case <-ticker.C:
			c.mu.Lock()
			if c.conn == nil {
				c.mu.Unlock()
				return
			}

			if err := c.conn.SetWriteDeadline(time.Now().Add(writeWait)); err != nil {
				c.logger.Printf("Erro ao definir write deadline para ping: %v", err)
				c.mu.Unlock()
				return
			}

			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				c.logger.Printf("Erro ao enviar ping: %v", err)
				c.mu.Unlock()
				return
			}
			c.mu.Unlock()
		}
	}
}

// método Connect()
func (c *WSClient) Connect() error {
	c.mu.Lock()
	if c.state == StateConnected {
		c.mu.Unlock()
		return nil
	}
	c.state = StateConnecting
	c.mu.Unlock()

	// 1. Primeiro faz o handshake HTTP
	handshakeURL := c.getHandshakeURL()
	c.logger.Printf("Tentando handshake com URL: %s", handshakeURL)

	handshakeResp, err := c.performHandshake(handshakeURL)
	if err != nil {
		return fmt.Errorf("falha no handshake: %v", err)
	}

	if handshakeResp == nil || handshakeResp.Sid == "" {
		return fmt.Errorf("handshake retornou resposta inválida ou SID vazio")
	}

	c.logger.Printf("Handshake bem sucedido, SID: %s", handshakeResp.Sid)

	// 2. Constrói a URL do WebSocket com o SID obtido
	wsURL, err := c.buildWebSocketURL(handshakeResp.Sid)
	if err != nil {
		return fmt.Errorf("erro ao construir URL do WebSocket: %v", err)
	}

	c.logger.Printf("Tentando conexão WebSocket com URL: %s", wsURL.String())

	// 3. Configura o dialer com os timeouts ajustados
	dialer := websocket.Dialer{
		HandshakeTimeout: 15 * time.Second, // Reduzido para 15 segundos
		Proxy:            http.ProxyFromEnvironment,
		TLSClientConfig: &tls.Config{
			MinVersion:         tls.VersionTLS12,
			InsecureSkipVerify: c.useSSL,
		},
	}

	// 4. Headers aprimorados mas mantendo a simplicidade
	headers := http.Header{
		"User-Agent":    []string{"Go-SocketIO-Client/1.0"},
		"Accept":        []string{"*/*"},
		"X-Api-Key":     []string{c.apiKey},
		"X-Clinic-ID":   []string{c.clinicID},
		"X-Instance-ID": []string{c.instanceID},
	}

	if c.apiKey != "" {
		headers.Set("X-Api-Key", c.apiKey)
	}

	// 5. Estabelece a conexão WebSocket com retry e backoff exponencial
	var conn *websocket.Conn
	var resp *http.Response
	maxRetries := 3
	baseDelay := time.Second

	for i := 0; i < maxRetries; i++ {
		conn, resp, err = dialer.Dial(wsURL.String(), headers)
		if err == nil {
			break
		}

		c.logger.Printf("Tentativa %d falhou: %v", i+1, err)

		if resp != nil && resp.Body != nil {
			if bodyBytes, readErr := io.ReadAll(resp.Body); readErr == nil {
				c.logger.Printf("Resposta do servidor: %s", string(bodyBytes))
			}
			resp.Body.Close()
		}

		if i < maxRetries-1 {
			// Backoff exponencial
			delay := time.Duration(math.Pow(2, float64(i))) * baseDelay
			c.logger.Printf("Aguardando %v antes da próxima tentativa", delay)
			time.Sleep(delay)
		}
	}

	if err != nil {
		return fmt.Errorf("falha na conexão WebSocket após %d tentativas: %v", maxRetries, err)
	}

	if conn == nil {
		return fmt.Errorf("conexão WebSocket é nil após tentativas de conexão")
	}

	c.mu.Lock()
	c.conn = conn
	c.sessionID = handshakeResp.Sid
	c.state = StateConnected
	c.reconnectCount = 0
	c.mu.Unlock()

	// Configura handlers de ping/pong
	c.setupPingPong()

	// Inicia as goroutines de leitura e escrita
	go c.readPump()
	go c.writePump()

	c.logger.Printf("Conexão WebSocket estabelecida com sucesso")
	return nil
}

// Novos métodos auxiliares
func (c *WSClient) getHandshakeURL() string {
	// Primeiro, parse a URL base para remover o protocolo existente
	baseURL, err := url.Parse(c.serverURL)
	if err != nil {
		c.logger.Printf("Erro ao parsear URL base: %v", err)
		return ""
	}

	// Remove qualquer protocolo existente
	host := baseURL.Host
	if host == "" {
		host = baseURL.Path // caso a URL não tenha protocolo
	}

	// Constrói a URL do handshake
	scheme := "http"
	if c.useSSL {
		scheme = "https"
	}

	return fmt.Sprintf("%s://%s/socket.io/?EIO=%s&transport=polling&t=%d-%d",
		scheme,
		host,
		engineIOVersion,
		time.Now().Unix(),
		rand.Intn(100000))
}

func (c *WSClient) performHandshake(handshakeURL string) (*HandshakeResponse, error) {
	client := &http.Client{
		Timeout: defaultTimeout,
	}

	req, err := http.NewRequest("GET", handshakeURL, nil)
	if err != nil {
		return nil, err
	}

	if c.apiKey != "" {
		req.Header.Set("X-Api-Key", c.apiKey)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("handshake falhou com status: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// Remove o caractere "0" do início da resposta do Engine.IO
	if len(body) < 2 {
		return nil, fmt.Errorf("resposta do handshake inválida")
	}
	jsonStr := string(body[1:])

	var handshakeResp HandshakeResponse
	if err := json.Unmarshal([]byte(jsonStr), &handshakeResp); err != nil {
		return nil, err
	}

	return &handshakeResp, nil
}

func (c *WSClient) buildWebSocketURL(sid string) (*url.URL, error) {
	// Primeiro, parse a URL base para remover o protocolo existente
	baseURL, err := url.Parse(c.serverURL)
	if err != nil {
		return nil, fmt.Errorf("erro ao parsear URL base: %v", err)
	}

	// Remove qualquer protocolo existente
	host := baseURL.Host
	if host == "" {
		host = baseURL.Path // caso a URL não tenha protocolo
	}

	// Constrói a nova URL com o protocolo correto
	scheme := "ws"
	if c.useSSL {
		scheme = "wss"
	}

	wsURL := fmt.Sprintf("%s://%s/socket.io/?EIO=%s&transport=%s&sid=%s",
		scheme,
		host,
		engineIOVersion,
		socketIOTransport,
		sid)

	return url.Parse(wsURL)
}

// reconnectWithBackoff implementa reconexão com backoff exponencial
func (c *WSClient) reconnectWithBackoff() {
	c.mu.Lock()
	if c.reconnecting {
		c.mu.Unlock()
		return
	}
	c.reconnecting = true
	c.state = StateReconnecting
	c.mu.Unlock()

	backoff := c.initialBackoff
	for c.reconnectCount < c.maxRetries {
		c.reconnectCount++

		c.logger.Printf("Tentativa de reconexão %d/%d após %v",
			c.reconnectCount, c.maxRetries, backoff)

		time.Sleep(backoff)

		if err := c.Connect(); err == nil {
			c.mu.Lock()
			c.reconnecting = false
			c.mu.Unlock()
			return
		}

		// Backoff exponencial com jitter
		jitter := time.Duration(rand.Int63n(int64(backoff) / 2))
		backoff = time.Duration(float64(backoff)*1.5) + jitter

		if backoff > c.maxBackoff {
			backoff = c.maxBackoff
		}
	}

	c.logger.Printf("Número máximo de tentativas de reconexão atingido!")
}

// Novo método para configurar ping/pong
func (c *WSClient) setupPingPong() {
	// Aumenta o tempo de espera do pong para 120 segundos
	pongWait := 120 * time.Second
	pingPeriod := (pongWait * 9) / 10

	// Handler de Pong mais robusto
	c.conn.SetPongHandler(func(string) error {
		c.logger.Printf("Pong recebido")
		return c.conn.SetReadDeadline(time.Now().Add(pongWait))
	})

	// Inicia o ticker de ping em uma goroutine separada
	go func() {
		ticker := time.NewTicker(pingPeriod)
		defer ticker.Stop()

		for {
			select {
			case <-c.done:
				return
			case <-ticker.C:
				c.mu.Lock()
				if c.state != StateConnected || c.conn == nil {
					c.mu.Unlock()
					return
				}

				// Envia ping Socket.IO (2 é o código de ping)
				if err := c.write(websocket.TextMessage, []byte("2")); err != nil {
					c.logger.Printf("Erro ao enviar ping: %v", err)
					c.mu.Unlock()
					go c.reconnect()
					return
				}
				c.mu.Unlock()
			}
		}
	}()
}

// ping envia uma mensagem de ping
func (c *WSClient) ping() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn == nil {
		return nil
	}
	return c.conn.WriteControl(websocket.PingMessage, []byte{}, time.Now().Add(time.Second))
}

// reconnect tenta reconectar ao servidor
func (c *WSClient) reconnect() {
	c.mu.Lock()
	if c.reconnecting {
		c.mu.Unlock()
		return
	}
	c.reconnecting = true
	c.mu.Unlock()

	for {
		select {
		case <-c.done:
			return
		default:
			log.Printf("Tentando reconectar ao servidor...")
			c.Close()

			if err := c.Connect(); err != nil {
				log.Printf("Falha na reconexão: %v. Tentando novamente em %v...",
					err, c.reconnectTimer)
				time.Sleep(c.reconnectTimer)
				continue
			}

			c.mu.Lock()
			c.reconnecting = false
			c.mu.Unlock()
			return
		}
	}
}

// Adicione este método ao WSClient
func (c *WSClient) Receive() chan models.Message {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Se o canal ainda não foi inicializado, cria ele
	if c.receiveChan == nil {
		c.receiveChan = make(chan models.Message, 100) // buffer de 100 mensagens
	}

	return c.receiveChan
}

// SendMessage envia uma mensagem para o servidor
func (c *WSClient) SendMessage(message interface{}) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.state != StateConnected {
		return fmt.Errorf("cliente não está conectado")
	}

	// Converte para WebSocketMessage se necessário
	var wsMsg *WebSocketMessage
	switch m := message.(type) {
	case *WebSocketMessage:
		wsMsg = m
	default:
		// Cria novo WebSocketMessage se o input não for um
		data, err := json.Marshal(message)
		if err != nil {
			return fmt.Errorf("erro ao serializar mensagem: %v", err)
		}
		wsMsg = &WebSocketMessage{
			ID:      generateMessageID(),
			Payload: data,
			Type:    "message",
		}
	}

	// Configura ACK se necessário
	wsMsg.AckRequired = true
	if wsMsg.ID == "" {
		wsMsg.ID = generateMessageID()
	}

	// Registra mensagem para aguardar ACK
	c.ackHandler.WaitForAck(wsMsg)

	// Serializa a mensagem
	data, err := json.Marshal(wsMsg)
	if err != nil {
		return fmt.Errorf("erro ao serializar mensagem: %v", err)
	}

	data = []byte("2" + string(data))

	// Envia a mensagem
	if err := c.conn.WriteMessage(websocket.TextMessage, data); err != nil {
		return fmt.Errorf("erro ao enviar mensagem: %v", err)
	}

	// Atualiza estatísticas
	c.incrementMessagesSent()
	c.updateBytesStats(true, len(data))

	return nil
}

// readPump lê mensagens do servidor
func (c *WSClient) readPump() {
	defer func() {
		c.mu.Lock()
		if c.conn != nil {
			c.conn.Close()
		}
		c.state = StateDisconnected
		c.mu.Unlock()

		// Evita reconexão imediata
		time.Sleep(time.Second * 2)
		go c.reconnectWithBackoff()
	}()

	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))

	for {
		messageType, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				c.logger.Printf("Erro inesperado na leitura: %v", err)
			}
			return
		}

		// Reseta o deadline após cada mensagem
		c.conn.SetReadDeadline(time.Now().Add(pongWait))

		if messageType == websocket.TextMessage {
			// Processa mensagens Socket.IO
			if string(message) == "2" {
				// Responde pong (3) para ping (2)
				c.write(websocket.TextMessage, []byte("3"))
				continue
			} else if string(message) == "3" {
				// Recebeu pong, apenas loga
				c.logger.Printf("Pong Socket.IO recebido")
				continue
			}

			c.handleMessage(message)
		}
	}
}

// Método para registrar handlers
func (c *WSClient) RegisterMessageHandler(msgType MessageType, handler MessageHandler) {
	c.mu.Lock()
	defer c.mu.Unlock()
	RegisterHandler(msgType, handler)
}

// handleRetry processa a retentativa de uma mensagem
func (c *WSClient) handleRetry(msg *WebSocketMessage) error {
	if msg.RetryPolicy == nil {
		msg.RetryPolicy = DefaultRetryPolicy()
	}

	if msg.ShouldRetry() {
		// Atualiza o status de retry
		msg.UpdateRetryStatus(fmt.Errorf("falha no ACK"))

		// Agenda reenvio
		go func() {
			time.Sleep(time.Until(msg.RetryStatus.NextAttempt))
			if err := c.SendMessage(msg); err != nil {
				c.logger.Printf("Erro ao reenviar mensagem %s: %v", msg.ID, err)
			}
		}()
		return nil
	}

	// Se não deve mais tentar, loga o erro final
	c.logger.Printf("Mensagem %s falhou definitivamente após %d tentativas",
		msg.ID, msg.RetryStatus.Attempts)
	return fmt.Errorf("número máximo de tentativas atingido")
}

// isDone verifica se o cliente foi fechado intencionalmente
func (c *WSClient) isDone() bool {
	select {
	case <-c.done:
		return true
	default:
		return false
	}
}

// Close fecha a conexão
func (c *WSClient) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn != nil {
		c.conn.WriteControl(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""),
			time.Now().Add(time.Second))
		c.conn.Close()
		c.conn = nil
		c.state = StateDisconnected
	}

	// Fecha o canal de recebimento se existir
	if c.receiveChan != nil {
		close(c.receiveChan)
		c.receiveChan = nil
	}

	select {
	case <-c.done:
		// Canal já está fechado
	default:
		close(c.done)
	}
}

// IsConnected retorna o status da conexão
func (c *WSClient) IsConnected() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.state == StateConnected
}

// Função auxiliar para gerar IDs únicos
func generateMessageID() string {
	return fmt.Sprintf("msg_%d_%d", time.Now().UnixNano(), rand.Int63())
}

// GetState retorna o estado atual da conexão
func (c *WSClient) GetState() ConnectionState {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.state
}

// GetLastError retorna o último erro ocorrido
func (c *WSClient) GetLastError() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.lastError
}

// SendMessageWithTimeout envia uma mensagem e aguarda ACK com timeout
func (c *WSClient) SendMessageWithTimeout(message interface{}, timeout time.Duration) error {
	c.mu.Lock()
	if c.state != StateConnected {
		c.mu.Unlock()
		return fmt.Errorf("cliente não está conectado")
	}
	c.mu.Unlock()

	// Converte para WebSocketMessage se necessário
	var wsMsg *WebSocketMessage
	switch m := message.(type) {
	case *WebSocketMessage:
		wsMsg = m
	default:
		// Cria novo WebSocketMessage se o input não for um
		data, err := json.Marshal(message)
		if err != nil {
			return fmt.Errorf("erro ao serializar mensagem: %v", err)
		}
		wsMsg = &WebSocketMessage{
			ID:      generateMessageID(),
			Payload: data,
			Type:    "message",
		}
	}

	// Configura ACK
	wsMsg.AckRequired = true
	if wsMsg.ID == "" {
		wsMsg.ID = generateMessageID()
	}

	// Cria canal para aguardar ACK
	ackChan := make(chan struct{})
	c.ackHandler.RegisterAckChannel(wsMsg.ID, ackChan)
	defer c.ackHandler.UnregisterAckChannel(wsMsg.ID)

	// Envia a mensagem
	if err := c.SendMessage(wsMsg); err != nil {
		return fmt.Errorf("erro ao enviar mensagem: %v", err)
	}

	// Aguarda ACK com timeout
	select {
	case <-ackChan:
		return nil
	case <-time.After(timeout):
		return fmt.Errorf("timeout aguardando ACK para mensagem %s", wsMsg.ID)
	}
}

// GetStats retorna uma cópia das estatísticas atuais
func (c *WSClient) GetStats() WSStats {
	c.statsMu.RLock()
	defer c.statsMu.RUnlock()
	return c.stats
}

// Métodos internos para atualizar as estatísticas
func (c *WSClient) incrementMessagesSent() {
	c.statsMu.Lock()
	c.stats.MessagesSent++
	c.statsMu.Unlock()
}

func (c *WSClient) incrementMessagesReceived() {
	c.statsMu.Lock()
	c.stats.MessagesReceived++
	c.statsMu.Unlock()
}

func (c *WSClient) updateBytesStats(sent bool, bytes int) {
	c.statsMu.Lock()
	if sent {
		c.stats.BytesSent += int64(bytes)
	} else {
		c.stats.BytesReceived += int64(bytes)
	}
	c.statsMu.Unlock()
}

func (c *WSClient) recordReconnection() {
	c.statsMu.Lock()
	c.stats.Reconnections++
	c.stats.LastReconnectAt = time.Now()
	c.statsMu.Unlock()
}

func (c *WSClient) ResetStats() {
	c.statsMu.Lock()
	defer c.statsMu.Unlock()

	c.stats = WSStats{
		ConnectedSince: c.stats.ConnectedSince, // Mantém apenas o tempo de conexão inicial
	}
}

func (c *WSClient) handleHandshake(data []byte) error {
	// Remove o primeiro caractere (tipo da mensagem)
	if len(data) == 0 {
		return fmt.Errorf("mensagem vazia")
	}

	messageType := string(data[0])
	payload := data[1:]

	switch messageType {
	case CONNECT:
		var handshake struct {
			Sid string `json:"sid"`
			// outros campos do handshake
		}
		if err := json.Unmarshal(payload, &handshake); err != nil {
			return err
		}
		c.sessionID = handshake.Sid
		c.logger.Printf("Handshake completo, sid: %s", c.sessionID)
	}
	return nil
}

// Método para escrever mensagens WebSocket de forma segura
func (c *WSClient) write(messageType int, payload []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn == nil {
		return fmt.Errorf("conexão WebSocket não está disponível")
	}

	c.conn.SetWriteDeadline(time.Now().Add(writeWait))
	return c.conn.WriteMessage(messageType, payload)
}

// Método para processar mensagens recebidas
func (c *WSClient) handleMessage(message []byte) {
	// Log da mensagem para debug
	c.logger.Printf("Processando mensagem: %s", string(message))

	// Aqui você pode adicionar a lógica específica para tratar diferentes tipos de mensagens
	// Por exemplo, verificar se é uma mensagem de evento Socket.IO e processá-la adequadamente

	// Exemplo básico de processamento de mensagem Socket.IO
	if len(message) > 0 {
		switch message[0] {
		case '0': // Connect
			c.logger.Printf("Mensagem de conexão Socket.IO recebida")
		case '2': // Ping
			c.logger.Printf("Ping Socket.IO recebido")
			c.write(websocket.TextMessage, []byte("3")) // Responde com pong
		case '3': // Pong
			c.logger.Printf("Pong Socket.IO recebido")
		case '4': // Message
			c.logger.Printf("Mensagem Socket.IO recebida: %s", string(message[1:]))
		case '5': // Upgrade
			c.logger.Printf("Solicitação de upgrade Socket.IO recebida")
		default:
			c.logger.Printf("Mensagem Socket.IO não reconhecida: %s", string(message))
		}
	}
}

func (c *WSClient) startHeartbeat() {
	ticker := time.NewTicker(25 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-c.done:
			return
		case <-ticker.C:
			// Envia ping Socket.IO (2probe)
			if err := c.write(websocket.TextMessage, []byte("2probe")); err != nil {
				c.logger.Printf("Erro no heartbeat: %v", err)
				go c.reconnect()
				return
			}
		}
	}
}
