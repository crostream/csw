package csw

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"strings"
)

const (
	privateKeyPEMType   = "PRIVATE KEY"
	publicKeyPEMType    = "PUBLIC KEY"
	maxPrivateKeyLength = 16 << 10
)

// GenerateKey makes a gallery-signing keypair: the private key as PKCS#8
// PEM and the public key as PKIX PEM. A nil r uses crypto/rand.
func GenerateKey(r io.Reader) (privPEM, pubPEM []byte, err error) {
	if r == nil {
		r = rand.Reader
	}
	pub, priv, err := ed25519.GenerateKey(r)
	if err != nil {
		return nil, nil, fmt.Errorf("csw: generate key: %w", err)
	}
	privDER, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		return nil, nil, fmt.Errorf("csw: encode private key: %w", err)
	}
	pubDER, err := x509.MarshalPKIXPublicKey(pub)
	if err != nil {
		return nil, nil, fmt.Errorf("csw: encode public key: %w", err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: privateKeyPEMType, Bytes: privDER}),
		pem.EncodeToMemory(&pem.Block{Type: publicKeyPEMType, Bytes: pubDER}), nil
}

// ParsePrivateKey reads an Ed25519 private key: PKCS#8 PEM, or base64 of
// the raw 64-byte key or 32-byte seed (for keys kept in a secret store).
func ParsePrivateKey(raw []byte) (ed25519.PrivateKey, error) {
	if len(raw) > maxPrivateKeyLength {
		return nil, errors.New("csw: private key file is too large")
	}
	if block, _ := pem.Decode(raw); block != nil {
		k, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("csw: parse private key: %w", err)
		}
		priv, ok := k.(ed25519.PrivateKey)
		if !ok {
			return nil, fmt.Errorf("csw: private key is a %T, want Ed25519", k)
		}
		return priv, nil
	}
	b, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(raw)))
	if err == nil {
		switch len(b) {
		case ed25519.PrivateKeySize:
			return ed25519.PrivateKey(b), nil
		case ed25519.SeedSize:
			return ed25519.NewKeyFromSeed(b), nil
		}
	}
	return nil, errors.New("csw: not a PEM or base64 Ed25519 private key")
}

// ParsePublicKey reads an Ed25519 public key: PKIX PEM or DER, the raw 32
// bytes, or their base64.
func ParsePublicKey(raw []byte) (ed25519.PublicKey, error) {
	if len(raw) == ed25519.PublicKeySize {
		return ed25519.PublicKey(raw), nil
	}
	raw = bytes.TrimSpace(raw)
	if b, err := base64.StdEncoding.DecodeString(string(raw)); err == nil && len(raw) > 0 {
		raw = b
	}
	if len(raw) == ed25519.PublicKeySize {
		return ed25519.PublicKey(raw), nil
	}
	if block, _ := pem.Decode(raw); block != nil {
		raw = block.Bytes
	}
	k, err := x509.ParsePKIXPublicKey(raw)
	if err != nil {
		return nil, fmt.Errorf("csw: parse public key: %w", err)
	}
	pub, ok := k.(ed25519.PublicKey)
	if !ok {
		return nil, fmt.Errorf("csw: public key is a %T, want Ed25519", k)
	}
	return pub, nil
}
