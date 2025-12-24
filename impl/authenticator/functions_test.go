package authenticator

import (
	"context"
	"evsys-back/entity"
	database_mock "evsys-back/impl/database-mock"
	"log/slog"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"golang.org/x/crypto/bcrypt"
)

func TestGeneratePasswordHash(t *testing.T) {
	db := database_mock.NewMockDB()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	auth := New(logger, db)

	t.Run("generates valid bcrypt hash", func(t *testing.T) {
		password := "testpassword123"
		hash := auth.generatePasswordHash(password)

		assert.NotEmpty(t, hash)
		assert.NotEqual(t, password, hash)

		// Verify hash is valid bcrypt
		err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
		assert.NoError(t, err)
	})

	t.Run("different passwords produce different hashes", func(t *testing.T) {
		hash1 := auth.generatePasswordHash("password1")
		hash2 := auth.generatePasswordHash("password2")

		assert.NotEqual(t, hash1, hash2)
	})

	t.Run("same password produces different hashes (salt)", func(t *testing.T) {
		hash1 := auth.generatePasswordHash("samepassword")
		hash2 := auth.generatePasswordHash("samepassword")

		// Bcrypt includes random salt, so hashes should differ
		assert.NotEqual(t, hash1, hash2)

		// But both should validate against the password
		err1 := bcrypt.CompareHashAndPassword([]byte(hash1), []byte("samepassword"))
		err2 := bcrypt.CompareHashAndPassword([]byte(hash2), []byte("samepassword"))
		assert.NoError(t, err1)
		assert.NoError(t, err2)
	})
}

func TestGenerateKey(t *testing.T) {
	db := database_mock.NewMockDB()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	auth := New(logger, db)

	t.Run("generates key of correct length", func(t *testing.T) {
		lengths := []int{5, 10, 20, 32}
		for _, length := range lengths {
			key := auth.generateKey(length)
			assert.Len(t, key, length)
		}
	})

	t.Run("generates hex-encoded string", func(t *testing.T) {
		key := auth.generateKey(20)
		// All characters should be valid hex (0-9, a-f)
		for _, c := range key {
			isHex := (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')
			assert.True(t, isHex, "character %c should be valid hex", c)
		}
	})

	t.Run("generates unique keys", func(t *testing.T) {
		keys := make(map[string]bool)
		for i := 0; i < 100; i++ {
			key := auth.generateKey(32)
			assert.False(t, keys[key], "key should be unique")
			keys[key] = true
		}
	})
}

func TestGetUserId(t *testing.T) {
	db := database_mock.NewMockDB()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	auth := New(logger, db)

	t.Run("generates 32-character user id", func(t *testing.T) {
		userId := auth.getUserId(context.Background())
		assert.Len(t, userId, 32)
	})

	t.Run("generates unique user ids", func(t *testing.T) {
		ids := make(map[string]bool)
		for i := 0; i < 50; i++ {
			id := auth.getUserId(context.Background())
			assert.False(t, ids[id], "user id should be unique")
			ids[id] = true
		}
	})
}

func TestUpdateLastSeen(t *testing.T) {
	db := database_mock.NewMockDB()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	auth := New(logger, db)

	t.Run("updates last seen without error", func(t *testing.T) {
		user := &entity.User{
			Username: "testuser",
			UserId:   "user123",
		}
		db.SeedUser(user)

		err := auth.updateLastSeen(context.Background(), user)
		assert.NoError(t, err)
	})
}
