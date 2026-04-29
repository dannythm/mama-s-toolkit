// Package crypto provides AES-CBC encrypt/decrypt for MasterMemory .bin.e files.
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"fmt"
)

var (
	// DefaultKey is the AES-128 key used by the game client.
	DefaultKey = []byte("6Cb01321EE5e6bBe")
	// DefaultIV is the AES-128 IV used by the game client.
	DefaultIV = []byte("EfcAef4CAe5f6DaA")
)

// Decrypt decrypts AES-CBC data with PKCS7 unpadding.
func Decrypt(data, key, iv []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("aes.NewCipher: %w", err)
	}
	if len(data)%aes.BlockSize != 0 {
		return nil, fmt.Errorf("ciphertext length %d is not a multiple of block size %d", len(data), aes.BlockSize)
	}

	mode := cipher.NewCBCDecrypter(block, iv)
	plaintext := make([]byte, len(data))
	mode.CryptBlocks(plaintext, data)

	// PKCS7 unpad
	padSize := int(plaintext[len(plaintext)-1])
	if padSize < 1 || padSize > aes.BlockSize {
		return nil, fmt.Errorf("invalid PKCS7 padding byte: %d", padSize)
	}
	for i := len(plaintext) - padSize; i < len(plaintext); i++ {
		if plaintext[i] != byte(padSize) {
			return nil, fmt.Errorf("invalid PKCS7 padding at byte %d", i)
		}
	}
	return plaintext[:len(plaintext)-padSize], nil
}

// Encrypt encrypts data with AES-CBC and PKCS7 padding.
func Encrypt(data, key, iv []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("aes.NewCipher: %w", err)
	}

	// PKCS7 pad
	padSize := aes.BlockSize - (len(data) % aes.BlockSize)
	padded := make([]byte, len(data)+padSize)
	copy(padded, data)
	for i := len(data); i < len(padded); i++ {
		padded[i] = byte(padSize)
	}

	mode := cipher.NewCBCEncrypter(block, iv)
	ciphertext := make([]byte, len(padded))
	mode.CryptBlocks(ciphertext, padded)
	return ciphertext, nil
}
