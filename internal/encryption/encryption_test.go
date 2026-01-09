package encryption

import (
	"testing"
)

func TestNewService(t *testing.T) {
	tests := []struct {
		name    string
		keySize int
		wantErr bool
	}{
		{"valid 32-byte key", 32, false},
		{"invalid 16-byte key", 16, true},
		{"invalid 24-byte key", 24, true},
		{"invalid 0-byte key", 0, true},
		{"invalid 64-byte key", 64, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key := make([]byte, tt.keySize)
			_, err := NewService(key)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewService() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestEncryptDecrypt(t *testing.T) {
	key, err := GenerateKey()
	if err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}

	svc, err := NewService(key)
	if err != nil {
		t.Fatalf("Failed to create service: %v", err)
	}

	tests := []struct {
		name      string
		plaintext string
	}{
		{"simple text", "hello world"},
		{"empty string", ""},
		{"special characters", "password!@#$%^&*()_+-=[]{}|;':\",./<>?"},
		{"unicode", "こんにちは世界 🌍"},
		{"long text", "Lorem ipsum dolor sit amet, consectetur adipiscing elit. " +
			"Sed do eiusmod tempor incididunt ut labore et dolore magna aliqua. " +
			"Ut enim ad minim veniam, quis nostrud exercitation ullamco laboris."},
		{"secret key", "sk-1234567890abcdefghijklmnopqrstuvwxyz"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Encrypt
			encrypted, err := svc.Encrypt(tt.plaintext)
			if err != nil {
				t.Fatalf("Encrypt() error = %v", err)
			}

			// Empty plaintext should result in empty ciphertext
			if tt.plaintext == "" && encrypted != "" {
				t.Errorf("Expected empty encrypted value for empty plaintext, got %q", encrypted)
			}

			// Non-empty plaintext should be encrypted
			if tt.plaintext != "" && encrypted == tt.plaintext {
				t.Errorf("Encrypted value should differ from plaintext")
			}

			// Decrypt
			decrypted, err := svc.Decrypt(encrypted)
			if err != nil {
				t.Fatalf("Decrypt() error = %v", err)
			}

			// Verify decryption matches original
			if decrypted != tt.plaintext {
				t.Errorf("Decrypt() = %q, want %q", decrypted, tt.plaintext)
			}
		})
	}
}

func TestEncryptionUniqueness(t *testing.T) {
	key, err := GenerateKey()
	if err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}

	svc, err := NewService(key)
	if err != nil {
		t.Fatalf("Failed to create service: %v", err)
	}

	plaintext := "test value"

	// Encrypt the same plaintext multiple times
	encrypted1, err := svc.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}

	encrypted2, err := svc.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}

	// Each encryption should produce a different ciphertext due to random nonces
	if encrypted1 == encrypted2 {
		t.Errorf("Encrypting same plaintext twice should produce different ciphertexts")
	}

	// But both should decrypt to the same plaintext
	decrypted1, err := svc.Decrypt(encrypted1)
	if err != nil {
		t.Fatalf("Decrypt() error = %v", err)
	}

	decrypted2, err := svc.Decrypt(encrypted2)
	if err != nil {
		t.Fatalf("Decrypt() error = %v", err)
	}

	if decrypted1 != plaintext || decrypted2 != plaintext {
		t.Errorf("Both decryptions should match original plaintext")
	}
}

func TestDecryptInvalid(t *testing.T) {
	key, err := GenerateKey()
	if err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}

	svc, err := NewService(key)
	if err != nil {
		t.Fatalf("Failed to create service: %v", err)
	}

	tests := []struct {
		name      string
		encrypted string
		wantErr   bool
	}{
		{"invalid base64", "not-valid-base64!@#$", true},
		{"too short", "YWJj", true},
		{"tampered data", "YWJjZGVmZ2hpamtsbW5vcHFyc3R1dnd4eXo=", true},
		{"empty string", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := svc.Decrypt(tt.encrypted)
			if (err != nil) != tt.wantErr {
				t.Errorf("Decrypt() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestDifferentKeysCannotDecrypt(t *testing.T) {
	key1, err := GenerateKey()
	if err != nil {
		t.Fatalf("Failed to generate key1: %v", err)
	}

	key2, err := GenerateKey()
	if err != nil {
		t.Fatalf("Failed to generate key2: %v", err)
	}

	svc1, err := NewService(key1)
	if err != nil {
		t.Fatalf("Failed to create service1: %v", err)
	}

	svc2, err := NewService(key2)
	if err != nil {
		t.Fatalf("Failed to create service2: %v", err)
	}

	plaintext := "secret data"

	// Encrypt with service1
	encrypted, err := svc1.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}

	// Try to decrypt with service2 (different key)
	_, err = svc2.Decrypt(encrypted)
	if err == nil {
		t.Errorf("Decrypt() with different key should fail")
	}
}

func TestGenerateKey(t *testing.T) {
	key1, err := GenerateKey()
	if err != nil {
		t.Fatalf("GenerateKey() error = %v", err)
	}

	if len(key1) != 32 {
		t.Errorf("GenerateKey() generated key of length %d, want 32", len(key1))
	}

	key2, err := GenerateKey()
	if err != nil {
		t.Fatalf("GenerateKey() error = %v", err)
	}

	// Each generated key should be unique
	if string(key1) == string(key2) {
		t.Errorf("GenerateKey() should produce unique keys")
	}
}
