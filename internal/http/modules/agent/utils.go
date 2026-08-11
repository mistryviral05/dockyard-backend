package agent

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"fmt"
)

const (
	tokenPrefix      = "agt_"
	tokenByteSize    = 32 
	defaultAgentPort = "8999"

	serverURL = "http://localhost:8080"
)

func GenerateToken() (plain string, hashed string, err error) {
	buf := make([]byte, tokenByteSize)
	if _, err := rand.Read(buf); err != nil {
		return "", "", fmt.Errorf("generate token: %w", err)
	}
	plain = tokenPrefix + base64.RawURLEncoding.EncodeToString(buf)
	return plain, HashToken(plain), nil
}

func HashToken(plain string) string {
	sum := sha256.Sum256([]byte(plain))
	return hex.EncodeToString(sum[:])
}

func CompareToken(plain, storedHash string) bool {
	return subtle.ConstantTimeCompare([]byte(HashToken(plain)), []byte(storedHash)) == 1
}

func BuildRunCommand(token, port string) string {
	return fmt.Sprintf(
		"curl -fsSL %s/api/v1/agents/install.sh | bash -s -- --token %s --server %s --port %s",
		serverURL, token, serverURL, port,
	)
}

func normalizePort(p string) string {
	if p == "" {
		return defaultAgentPort
	}
	return p
}
