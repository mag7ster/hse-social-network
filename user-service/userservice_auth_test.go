package main

import (
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func generateTempKeys(t *testing.T) (privateKeyPath, publicKeyPath string) {
	t.Helper()
	tempDir := t.TempDir()
	privPath := filepath.Join(tempDir, "temp_private.pem")
	pubPath := filepath.Join(tempDir, "temp_public.pem")

	cmd := exec.Command("openssl", "genrsa", "-out", privPath, "2048")
	if err := cmd.Run(); err != nil {
		t.Fatalf("Generating private key error: %v", err)
	}

	cmd = exec.Command("openssl", "rsa", "-in", privPath, "-pubout", "-out", pubPath)
	if err := cmd.Run(); err != nil {
		t.Fatalf("Generating private key error: %v", err)
	}

	return privPath, pubPath
}

func TestLoadRSAKeys(t *testing.T) {
	privPath, pubPath := generateTempKeys(t)
	LoadRSAKeys(privPath, pubPath)

	if rsaPrivateKey == nil || rsaPublicKey == nil {
		t.Error("RSA keys were not loaded correctly")
	}
}

func TestGenerateAndAuthenticateToken(t *testing.T) {
	privPath, pubPath := generateTempKeys(t)
	LoadRSAKeys(privPath, pubPath)

	user := &User{ID: 1, Role: "admin"}
	tokenStr, expTime, err := GenerateToken(user)
	if err != nil {
		t.Fatalf("Token generation error: %v", err)
	}

	if time.Now().After(expTime) {
		t.Error("The time of token is set in the past")
	}

	token, claims, err := Authenticate(tokenStr)
	if err != nil {
		t.Fatalf("Token authentication error: %v", err)
	}

	if !token.Valid {
		t.Error("The generated token is unavailable")
	}

	userID := GetUserIdByClaims(claims)
	if userID != user.ID {
		t.Errorf("Userid %d was expected, %d", user.ID, userID)
	}

	if claims["role"] != user.Role {
		t.Errorf("The role %s was expected, %v obtained", user.Role, claims["role"])
	}
}

func TestPasswordHashing(t *testing.T) {
	password := "securepassword"
	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("Password hashching error: %v", err)
	}

	if !CheckPasswordHash(password, hash) {
		t.Error("Hesh password does not match the source password")
	}

	if CheckPasswordHash("wrongpassword", hash) {
		t.Error("The wrong password incorrectly passed the check")
	}
}
