package authenticator

import (
	"crypto/rand"
	"encoding/hex"
	"evsys-back/entity"
	"fmt"
	"golang.org/x/crypto/bcrypt"
)

func (a *Authenticator) generatePasswordHash(password string) string {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		a.logger.Error("generating password hash", err)
		return ""
	}
	return string(hash)
}

// generate random key of specified length by reading cryptographic random function
// and converting bytes to hex encoding; ensures key length by slicing if necessary
func (a *Authenticator) generateKey(length int) string {
	b := make([]byte, length)
	_, err := rand.Read(b)
	if err != nil {
		return ""
	}
	s := hex.EncodeToString(b)
	// string length is doubled because of hex encoding
	if len(s) > length {
		s = s[:length]
	}
	return s
}

// generate user ID using a 32-character key, ensuring uniqueness by recursively checking database
// if user already exists with the generated ID
// Note: This recursive approach could lead to stack overflow if not managed properly
func (a *Authenticator) getUserId() string {
	id := a.generateKey(32)
	user, _ := a.database.GetUser(id)
	if user == nil {
		return id
	}
	return a.getUserId()
}

// update user last seen
func (a *Authenticator) updateLastSeen(user *entity.User) error {
	err := a.database.UpdateLastSeen(user)
	if err != nil {
		return fmt.Errorf("updating user last seen: %s", err)
	}
	return nil
}
