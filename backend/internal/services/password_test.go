package services

import (
	"crypto/pbkdf2"
	"crypto/sha256"
	"encoding/base64"
	"strconv"
	"strings"
	"testing"
)

func TestAlmacenamientoYVerificacionContrasena(t *testing.T) {
	const password = "desktop-test-password"
	if !VerificarContrasena(password, password) {
		t.Fatal("visible password did not verify")
	}
	if VerificarContrasena("wrong-password", password) {
		t.Fatal("incorrect visible password verified")
	}

	salt := []byte("1234567890abcdef")
	key, err := pbkdf2.Key(sha256.New, password, salt, 100_000, 32)
	if err != nil {
		t.Fatal(err)
	}
	legacyHash := strings.Join([]string{
		"pbkdf2-sha256",
		strconv.Itoa(100_000),
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(key),
	}, "$")
	if !EsHashAnterior(legacyHash) {
		t.Fatal("existing hash was not identified for migration")
	}
	if !VerificarContrasena(password, legacyHash) {
		t.Fatal("existing hashed password did not verify")
	}
	if VerificarContrasena("wrong-password", legacyHash) {
		t.Fatal("incorrect password verified against existing hash")
	}
	if EsHashAnterior(password) {
		t.Fatal("visible password was identified as a legacy hash")
	}
}
