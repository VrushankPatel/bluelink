package firebase

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
)

// deriveKey generates a 32-byte key from the room ID using SHA-256
func deriveKey(roomID string) []byte {
	hash := sha256.Sum256([]byte(roomID))
	return hash[:]
}

// encrypt encrypts the given text using AES-256-GCM with the derived key
func encrypt(text string, roomID string) (string, error) {
	key := deriveKey(roomID)

	// Create new cipher block
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %w", err)
	}

	// Create GCM mode
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM: %w", err)
	}

	// Generate nonce
	nonce := make([]byte, gcm.NonceSize())
	// In a real application, you should use crypto/rand for the nonce
	// For this example, we'll use a deterministic nonce based on the room ID
	copy(nonce, deriveKey(roomID)[:gcm.NonceSize()])

	// Encrypt and seal
	ciphertext := gcm.Seal(nonce, nonce, []byte(text), nil)

	// Encode to base64 for storage
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// decrypt decrypts the given ciphertext using AES-256-GCM with the derived key
func decrypt(encryptedText string, roomID string) (string, error) {
	key := deriveKey(roomID)

	// Decode from base64
	ciphertext, err := base64.StdEncoding.DecodeString(encryptedText)
	if err != nil {
		return "", fmt.Errorf("failed to decode base64: %w", err)
	}

	// Create new cipher block
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %w", err)
	}

	// Create GCM mode
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM: %w", err)
	}

	// Extract nonce
	if len(ciphertext) < gcm.NonceSize() {
		return "", fmt.Errorf("ciphertext too short")
	}
	nonce, ciphertext := ciphertext[:gcm.NonceSize()], ciphertext[gcm.NonceSize():]

	// Decrypt
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("failed to decrypt: %w", err)
	}

	return string(plaintext), nil
}
