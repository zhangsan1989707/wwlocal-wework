package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"errors"
	"fmt"
)

var (
	ErrInvalidIVLength    = errors.New("invalid IV length")
	ErrInvalidCiphertext2 = errors.New("ciphertext too short")
	ErrPaddingFailed      = errors.New("pkcs7 padding failed")
)

type AESDecryptor struct {
	key []byte
}

func NewAESDecryptor(key []byte) (*AESDecryptor, error) {
	if len(key) != 16 {
		return nil, fmt.Errorf("invalid AES key length: %d, expected 16", len(key))
	}
	return &AESDecryptor{key: key}, nil
}

func (a *AESDecryptor) Decrypt(ciphertext []byte) ([]byte, error) {
	if len(ciphertext) < aes.BlockSize {
		return nil, ErrInvalidCiphertext2
	}

	block, err := aes.NewCipher(a.key)
	if err != nil {
		return nil, fmt.Errorf("create AES cipher failed: %w", err)
	}

	iv := ciphertext[:aes.BlockSize]
	ciphertext = ciphertext[aes.BlockSize:]

	if len(ciphertext)%aes.BlockSize != 0 {
		return nil, fmt.Errorf("ciphertext length must be multiple of block size")
	}

	mode := cipher.NewCBCDecrypter(block, iv)
	mode.CryptBlocks(ciphertext, ciphertext)

	plaintext, err := pkcs7Unpad(ciphertext, aes.BlockSize)
	if err != nil {
		return nil, fmt.Errorf("unpad failed: %w", err)
	}

	return plaintext, nil
}

func pkcs7Unpad(data []byte, blockSize int) ([]byte, error) {
	if data == nil || len(data) == 0 {
		return nil, ErrPaddingFailed
	}

	if len(data)%blockSize != 0 {
		return nil, ErrPaddingFailed
	}

	padLen := int(data[len(data)-1])
	if padLen == 0 || padLen > blockSize {
		return nil, ErrPaddingFailed
	}

	for i := len(data) - padLen; i < len(data); i++ {
		if data[i] != byte(padLen) {
			return nil, ErrPaddingFailed
		}
	}

	return data[:len(data)-padLen], nil
}