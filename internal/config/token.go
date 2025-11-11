// internal/config/token.go
package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"
)

// TokenClaims representa os dados que serão codificados no token
type TokenClaims struct {
	ClinicID   string    `json:"clinic_id"`
	InstanceID string    `json:"instance_id"`
	Timestamp  time.Time `json:"timestamp"`
	Signature  string    `json:"signature"`
	ExpiresAt  time.Time `json:"expires_at"`
}

// TokenGenerator gerencia a criação de tokens
type TokenGenerator struct {
	secretKey string
}

// NewTokenGenerator cria uma nova instância do gerador de tokens
func NewTokenGenerator(secretKey string) *TokenGenerator {
	return &TokenGenerator{
		secretKey: secretKey,
	}
}

// GenerateToken cria um novo token para uma clínica
func (g *TokenGenerator) GenerateToken(clinicID, instanceID string) (string, error) {
	now := time.Now()

	// Cria os claims do token
	claims := TokenClaims{
		ClinicID:   clinicID,
		InstanceID: instanceID,
		Timestamp:  now,
		ExpiresAt:  now.Add(24 * time.Hour), // Token válido por 24h
	}

	// Gera a assinatura
	signature, err := g.generateSignature(claims)
	if err != nil {
		return "", fmt.Errorf("erro ao gerar assinatura: %v", err)
	}
	claims.Signature = signature

	// Codifica os claims em JSON
	jsonData, err := json.Marshal(claims)
	if err != nil {
		return "", fmt.Errorf("erro ao codificar claims: %v", err)
	}

	// Codifica em base64
	token := base64.URLEncoding.EncodeToString(jsonData)
	return token, nil
}

// generateSignature cria uma assinatura HMAC dos claims
func (g *TokenGenerator) generateSignature(claims TokenClaims) (string, error) {
	// String para assinar: clinicID + instanceID + timestamp
	dataToSign := fmt.Sprintf("%s:%s:%d",
		claims.ClinicID,
		claims.InstanceID,
		claims.Timestamp.Unix(),
	)

	// Cria HMAC
	h := hmac.New(sha256.New, []byte(g.secretKey))
	h.Write([]byte(dataToSign))

	// Retorna assinatura em base64
	return base64.URLEncoding.EncodeToString(h.Sum(nil)), nil
}

// ValidateToken verifica se um token é válido
func (g *TokenGenerator) ValidateToken(token string) (*TokenClaims, error) {
	// Decodifica o token
	jsonData, err := base64.URLEncoding.DecodeString(token)
	if err != nil {
		return nil, fmt.Errorf("token inválido: %v", err)
	}

	// Decodifica os claims
	var claims TokenClaims
	if err := json.Unmarshal(jsonData, &claims); err != nil {
		return nil, fmt.Errorf("erro ao decodificar claims: %v", err)
	}

	// Verifica expiração
	if time.Now().After(claims.ExpiresAt) {
		return nil, fmt.Errorf("token expirado")
	}

	// Verifica assinatura
	expectedSignature, err := g.generateSignature(claims)
	if err != nil {
		return nil, fmt.Errorf("erro ao gerar assinatura: %v", err)
	}

	if claims.Signature != expectedSignature {
		return nil, fmt.Errorf("assinatura inválida")
	}

	return &claims, nil
}
