package crypto

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"testing"
)

func generateTestKey(t *testing.T) string {
	privKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("Failed to generate test key: %v", err)
	}

	privPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privKey),
	})

	return string(privPEM)
}

func TestNewRSADecryptor(t *testing.T) {
	keyPEM := generateTestKey(t)
	dec, err := NewRSADecryptor(keyPEM)
	if err != nil {
		t.Fatalf("NewRSADecryptor failed: %v", err)
	}
	if dec == nil {
		t.Fatal("NewRSADecryptor returned nil decryptor")
	}
	if dec.GetPrivateKey() == nil {
		t.Fatal("GetPrivateKey returned nil")
	}
}

func TestNewRSADecryptor_InvalidPEM(t *testing.T) {
	_, err := NewRSADecryptor("invalid pem")
	if err == nil {
		t.Fatal("Expected error for invalid PEM, got nil")
	}
}

func TestAESDecryptor(t *testing.T) {
	// 简单的 AES 解密器创建测试
	key := make([]byte, 16) // AES-128
	for i := 0; i < 16; i++ {
		key[i] = byte(i)
	}
	_, err := NewAESDecryptor(key)
	if err != nil {
		t.Fatalf("NewAESDecryptor failed: %v", err)
	}

	// 测试无效密钥长度
	_, err = NewAESDecryptor(make([]byte, 8))
	if err == nil {
		t.Fatal("Expected error for invalid key length, got nil")
	}
}

func TestRSADecryptor_InvalidCiphertext(t *testing.T) {
	keyPEM := generateTestKey(t)
	dec, err := NewRSADecryptor(keyPEM)
	if err != nil {
		t.Fatalf("NewRSADecryptor failed: %v", err)
	}

	// 测试无效 Base64
	_, err = dec.Decrypt("invalid-base64")
	if err == nil {
		t.Fatal("Expected error for invalid base64, got nil")
	}
}
