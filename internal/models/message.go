// internal/models/message.go
package models

import "time"

type MessageType string

const (
	TypeNewRecord    MessageType = "NEW_RECORD"
	TypeUpdateRecord MessageType = "UPDATE_RECORD"
	TypeAck          MessageType = "ACK"
	TypeError        MessageType = "ERROR"
)

type Message struct {
	ID        string      `json:"id"`
	Type      MessageType `json:"type"`
	Timestamp time.Time   `json:"timestamp"`
	Payload   Payload     `json:"payload"`
	Status    string      `json:"status,omitempty"`
}

type Payload struct {
	TableName string                 `json:"table_name,omitempty"`
	Action    string                 `json:"action,omitempty"`
	Data      map[string]interface{} `json:"data,omitempty"`
	Error     string                 `json:"error,omitempty"`
}
