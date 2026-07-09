// Package utils — crypto: AES-GCM symmetric encryption for secrets at rest
// (TOTP secrets, per TDD §5.3). The encryption key is derived from an app-level
// secret (TWO_FA_ENCRYPTION_KEY) — it never lives in the DB, so a DB dump
// alone cannot recover the encrypted secrets.
package utils

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
)

// Encrypt encrypts plaintext with AES-256-GCM using a key derived from
// `keyMaterial` (SHA-256 → 32-byte AES key). Returns nonce||ciphertext. The
// same key must be passed to Decrypt.
func Encrypt(keyMaterial []byte, plaintext []byte) ([]byte, error) {
	aead, err := newAEAD(keyMaterial)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("utils: encrypt: read nonce: %w", err)
	}
	// Seal appends the ciphertext+tag to the nonce so the output is nonce||ct.
	return aead.Seal(nonce, nonce, plaintext, nil), nil
}

// Decrypt reverses Encrypt. Returns an error if the key or ciphertext is wrong.
func Decrypt(keyMaterial []byte, blob []byte) ([]byte, error) {
	aead, err := newAEAD(keyMaterial)
	if err != nil {
		return nil, err
	}
	if len(blob) < aead.NonceSize() {
		return nil, errors.New("utils: decrypt: ciphertext too short")
	}
	nonce, ct := blob[:aead.NonceSize()], blob[aead.NonceSize():]
	return aead.Open(nil, nonce, ct, nil)
}

func newAEAD(keyMaterial []byte) (cipher.AEAD, error) {
	key := sha256.Sum256(keyMaterial) // derive a fixed 32-byte AES-256 key
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, fmt.Errorf("utils: new cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("utils: new gcm: %w", err)
	}
	return aead, nil
}
