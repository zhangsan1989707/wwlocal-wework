package crypto

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
)

var (
	ErrInvalidKeySize    = errors.New("invalid key size")
	ErrDecryptFailed     = errors.New("decrypt failed")
	ErrInvalidCiphertext = errors.New("invalid ciphertext")
)

type RSADecryptor struct {
	privateKey *rsa.PrivateKey
}

func NewRSADecryptor(privateKeyPEM string) (*RSADecryptor, error) {
	block, _ := pem.Decode([]byte(privateKeyPEM))
	if block == nil {
		return nil, errors.New("invalid PEM format")
	}

	priv, err := parsePrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse private key failed: %w", err)
	}

	return &RSADecryptor{privateKey: priv}, nil
}

func NewRSADecryptorFromFile(path string) (*RSADecryptor, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read key file failed: %w", err)
	}
	return NewRSADecryptor(string(data))
}

func (r *RSADecryptor) Decrypt(encryptedBase64 string) ([]byte, error) {
	ciphertext, err := base64.StdEncoding.DecodeString(encryptedBase64)
	if err != nil {
		return nil, fmt.Errorf("base64 decode failed: %w", err)
	}

	if len(ciphertext) > r.privateKey.N.BitLen()/8 {
		return nil, ErrInvalidCiphertext
	}

	plaintext, err := rsa.DecryptPKCS1v15(nil, r.privateKey, ciphertext)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrDecryptFailed, err)
	}

	return plaintext, nil
}

func (r *RSADecryptor) GetPrivateKey() *rsa.PrivateKey {
	return r.privateKey
}

func parsePrivateKey(der []byte) (*rsa.PrivateKey, error) {
	key, err := x509.ParsePKCS1PrivateKey(der)
	if err == nil {
		return key, nil
	}

	keyInterface, err := x509.ParsePKCS8PrivateKey(der)
	if err != nil {
		return nil, err
	}

	priv, ok := keyInterface.(*rsa.PrivateKey)
	if !ok {
		return nil, errors.New("not RSA private key")
	}

	return priv, nil
}