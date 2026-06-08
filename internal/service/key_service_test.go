package service

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"testing"
)

func TestValidateRSAPrivateKeyPEM(t *testing.T) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	pkcs1 := string(pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privateKey),
	}))
	pkcs8Bytes, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		t.Fatalf("marshal pkcs8: %v", err)
	}
	pkcs8 := string(pem.EncodeToMemory(&pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: pkcs8Bytes,
	}))

	tests := []struct {
		name    string
		pem     string
		wantErr bool
	}{
		{name: "pkcs1", pem: pkcs1},
		{name: "pkcs8", pem: pkcs8},
		{name: "not pem", pem: "not a pem", wantErr: true},
		{name: "not private key", pem: string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: []byte("bad")})), wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateRSAPrivateKeyPEM(tt.pem)
			if tt.wantErr {
				if !errors.Is(err, ErrInvalidPrivateKey) {
					t.Fatalf("got %v, want ErrInvalidPrivateKey", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("got %v, want nil", err)
			}
		})
	}
}

func TestValidKeyVersion(t *testing.T) {
	tests := []struct {
		version string
		want    bool
	}{
		{version: "v2", want: true},
		{version: "2026.06_prod-1", want: true},
		{version: "", want: false},
		{version: "../v2", want: false},
		{version: "v2/private", want: false},
		{version: "密钥v2", want: false},
	}

	for _, tt := range tests {
		if got := validKeyVersion(tt.version); got != tt.want {
			t.Fatalf("validKeyVersion(%q) = %v, want %v", tt.version, got, tt.want)
		}
	}
}
