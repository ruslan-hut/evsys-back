package authenticator

import (
	"context"
	"evsys-back/entity"
	database_mock "evsys-back/impl/database-mock"
	"golang.org/x/crypto/bcrypt"
	"log/slog"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockFirebase implements FirebaseAuth interface for testing
type mockFirebase struct {
	returnUserId string
	returnErr    error
}

func (m *mockFirebase) CheckToken(token string) (string, error) {
	return m.returnUserId, m.returnErr
}

func newTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
}

func createHashedPassword(password string) string {
	hash, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(hash)
}

func TestNew(t *testing.T) {
	logger := newTestLogger()

	t.Run("nil repository returns nil", func(t *testing.T) {
		auth := New(logger, nil)
		assert.Nil(t, auth)
	})

	t.Run("valid repository returns authenticator", func(t *testing.T) {
		db := database_mock.NewMockDB()
		auth := New(logger, db)
		assert.NotNil(t, auth)
	})
}

func TestAuthenticateByToken(t *testing.T) {
	tests := []struct {
		name         string
		token        string
		setup        func(*database_mock.MockDB, *Authenticator)
		wantErr      bool
		wantErrMsg   string
		wantUsername string
	}{
		{
			name:       "empty token",
			token:      "",
			setup:      nil,
			wantErr:    true,
			wantErrMsg: "empty token",
		},
		{
			name:       "token shorter than 32 chars",
			token:      "shorttoken",
			wantErr:    true,
			wantErrMsg: "invalid token",
		},
		{
			name:  "valid 32-char token in database",
			token: "12345678901234567890123456789012",
			setup: func(db *database_mock.MockDB, a *Authenticator) {
				db.SeedUser(&entity.User{
					Username: "testuser",
					UserId:   "user123",
					Token:    "12345678901234567890123456789012",
				})
			},
			wantErr:      false,
			wantUsername: "testuser",
		},
		{
			name:       "valid 32-char token not in database",
			token:      "abcdefgh901234567890123456789012",
			setup:      nil,
			wantErr:    true,
			wantErrMsg: "token check failed",
		},
		{
			name:  "firebase token - valid",
			token: "firebase_token_longer_than_32_characters_here",
			setup: func(db *database_mock.MockDB, a *Authenticator) {
				db.SeedUser(&entity.User{
					Username: "firebase_user",
					UserId:   "firebase_user_id",
				})
				a.SetFirebase(&mockFirebase{
					returnUserId: "firebase_user_id",
					returnErr:    nil,
				})
			},
			wantErr:      false,
			wantUsername: "firebase_user",
		},
		{
			name:  "firebase token - firebase error",
			token: "firebase_token_longer_than_32_characters_here",
			setup: func(db *database_mock.MockDB, a *Authenticator) {
				a.SetFirebase(&mockFirebase{
					returnUserId: "",
					returnErr:    assert.AnError,
				})
			},
			wantErr:    true,
			wantErrMsg: "firebase token check",
		},
		{
			name:       "long token but no firebase configured",
			token:      "long_token_without_firebase_configured_here",
			setup:      nil,
			wantErr:    true,
			wantErrMsg: "token check failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := database_mock.NewMockDB()
			logger := newTestLogger()
			auth := New(logger, db)
			require.NotNil(t, auth)

			if tt.setup != nil {
				tt.setup(db, auth)
			}

			user, err := auth.AuthenticateByToken(context.Background(), tt.token)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.wantErrMsg != "" {
					assert.Contains(t, err.Error(), tt.wantErrMsg)
				}
				return
			}

			assert.NoError(t, err)
			assert.NotNil(t, user)
			assert.Equal(t, tt.wantUsername, user.Username)
		})
	}
}

func TestAuthenticateUser(t *testing.T) {
	tests := []struct {
		name        string
		username    string
		password    string
		setup       func(*database_mock.MockDB)
		wantErr     bool
		wantErrMsg  string
		checkResult func(*testing.T, *entity.User)
	}{
		{
			name:     "valid credentials",
			username: "testuser",
			password: "correct_password",
			setup: func(db *database_mock.MockDB) {
				db.SeedUser(&entity.User{
					Username: "testuser",
					UserId:   "user123",
					Password: createHashedPassword("correct_password"),
				})
			},
			wantErr: false,
			checkResult: func(t *testing.T, user *entity.User) {
				assert.Equal(t, "testuser", user.Username)
				assert.Empty(t, user.Password, "password should be cleared")
				assert.NotEmpty(t, user.Token, "token should be generated")
				assert.Len(t, user.Token, tokenLength)
			},
		},
		{
			name:       "user not found",
			username:   "nonexistent",
			password:   "anypassword",
			setup:      nil,
			wantErr:    true,
			wantErrMsg: "user not found",
		},
		{
			name:     "wrong password",
			username: "testuser",
			password: "wrong_password",
			setup: func(db *database_mock.MockDB) {
				db.SeedUser(&entity.User{
					Username: "testuser",
					UserId:   "user123",
					Password: createHashedPassword("correct_password"),
				})
			},
			wantErr:    true,
			wantErrMsg: "password check failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := database_mock.NewMockDB()
			logger := newTestLogger()
			auth := New(logger, db)

			if tt.setup != nil {
				tt.setup(db)
			}

			user, err := auth.AuthenticateUser(context.Background(), tt.username, tt.password)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.wantErrMsg != "" {
					assert.Contains(t, err.Error(), tt.wantErrMsg)
				}
				return
			}

			assert.NoError(t, err)
			assert.NotNil(t, user)
			if tt.checkResult != nil {
				tt.checkResult(t, user)
			}
		})
	}
}

func TestRegisterUser(t *testing.T) {
	tests := []struct {
		name       string
		user       *entity.User
		setup      func(*database_mock.MockDB)
		wantErr    bool
		wantErrMsg string
	}{
		{
			name: "valid registration",
			user: &entity.User{
				Username: "newuser",
				Password: "securepassword",
				Token:    "VALIDCODE",
			},
			setup: func(db *database_mock.MockDB) {
				db.SeedInvite("VALIDCODE")
			},
			wantErr: false,
		},
		{
			name: "empty password",
			user: &entity.User{
				Username: "newuser",
				Password: "",
				Token:    "VALIDCODE",
			},
			wantErr:    true,
			wantErrMsg: "empty password",
		},
		{
			name: "empty username",
			user: &entity.User{
				Username: "",
				Password: "password",
				Token:    "VALIDCODE",
			},
			wantErr:    true,
			wantErrMsg: "empty username",
		},
		{
			name: "empty invite code",
			user: &entity.User{
				Username: "newuser",
				Password: "password",
				Token:    "",
			},
			wantErr:    true,
			wantErrMsg: "empty invite code",
		},
		{
			name: "username already exists",
			user: &entity.User{
				Username: "existinguser",
				Password: "password",
				Token:    "VALIDCODE",
			},
			setup: func(db *database_mock.MockDB) {
				db.SeedUser(&entity.User{Username: "existinguser", UserId: "user1"})
				db.SeedInvite("VALIDCODE")
			},
			wantErr:    true,
			wantErrMsg: "user already exists",
		},
		{
			name: "invalid invite code",
			user: &entity.User{
				Username: "newuser",
				Password: "password",
				Token:    "INVALIDCODE",
			},
			setup:      nil,
			wantErr:    true,
			wantErrMsg: "invalid invite code",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := database_mock.NewMockDB()
			logger := newTestLogger()
			auth := New(logger, db)

			if tt.setup != nil {
				tt.setup(db)
			}

			err := auth.RegisterUser(context.Background(), tt.user)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.wantErrMsg != "" {
					assert.Contains(t, err.Error(), tt.wantErrMsg)
				}
				return
			}

			assert.NoError(t, err)
			// Verify defaults were set
			assert.Equal(t, defaultPaymentPlan, tt.user.PaymentPlan)
			assert.Equal(t, defaultUserRole, tt.user.Role)
			assert.Equal(t, defaultUserGroupId, tt.user.Group)
			assert.NotEmpty(t, tt.user.UserId)
		})
	}
}

func TestGetUserTag(t *testing.T) {
	tests := []struct {
		name     string
		user     *entity.User
		setup    func(*database_mock.MockDB)
		wantErr  bool
		checkTag func(*testing.T, string)
	}{
		{
			name: "user has no tags - auto creates one",
			user: &entity.User{
				Username: "testuser",
				UserId:   "user123",
			},
			setup:   nil,
			wantErr: false,
			checkTag: func(t *testing.T, tag string) {
				assert.NotEmpty(t, tag)
				assert.Len(t, tag, 20, "auto-generated tag should be 20 chars")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := database_mock.NewMockDB()
			logger := newTestLogger()
			auth := New(logger, db)

			if tt.setup != nil {
				tt.setup(db)
			}

			tag, err := auth.GetUserTag(context.Background(), tt.user)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			if tt.checkTag != nil {
				tt.checkTag(t, tag)
			}
		})
	}
}

func TestCommandAccess(t *testing.T) {
	tests := []struct {
		name    string
		user    *entity.User
		command string
		wantErr bool
	}{
		{
			name:    "nil user",
			user:    nil,
			command: "RemoteStartTransaction",
			wantErr: true,
		},
		{
			name:    "admin can access any command",
			user:    &entity.User{Role: "admin"},
			command: "ChangeConfiguration",
			wantErr: false,
		},
		{
			name:    "admin can access user commands",
			user:    &entity.User{Role: "admin"},
			command: "RemoteStartTransaction",
			wantErr: false,
		},
		{
			name:    "regular user can access user commands",
			user:    &entity.User{Role: "user"},
			command: "RemoteStartTransaction",
			wantErr: false,
		},
		{
			name:    "regular user can access RemoteStopTransaction",
			user:    &entity.User{Role: "user"},
			command: "RemoteStopTransaction",
			wantErr: false,
		},
		{
			name:    "regular user denied admin command",
			user:    &entity.User{Role: "user"},
			command: "ChangeConfiguration",
			wantErr: true,
		},
		{
			name:    "operator (power user) can access non-admin commands",
			user:    &entity.User{Role: "operator"},
			command: "GetDiagnostics",
			wantErr: true, // GetDiagnostics is in adminCommands
		},
		{
			name:    "operator can access user commands",
			user:    &entity.User{Role: "operator"},
			command: "RemoteStartTransaction",
			wantErr: false,
		},
		{
			name:    "operator denied ChangeConfiguration",
			user:    &entity.User{Role: "operator"},
			command: "ChangeConfiguration",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := database_mock.NewMockDB()
			logger := newTestLogger()
			auth := New(logger, db)

			err := auth.CommandAccess(tt.user, tt.command)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestHasAccess(t *testing.T) {
	tests := []struct {
		name      string
		user      *entity.User
		subSystem string
		wantErr   bool
	}{
		{
			name:      "nil user",
			user:      nil,
			subSystem: "reports",
			wantErr:   true,
		},
		{
			name:      "admin has access",
			user:      &entity.User{Role: "admin"},
			subSystem: "reports",
			wantErr:   false,
		},
		{
			name:      "operator has access",
			user:      &entity.User{Role: "operator"},
			subSystem: "reports",
			wantErr:   false,
		},
		{
			name:      "regular user denied",
			user:      &entity.User{Role: "user"},
			subSystem: "reports",
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := database_mock.NewMockDB()
			logger := newTestLogger()
			auth := New(logger, db)

			err := auth.HasAccess(tt.user, tt.subSystem)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestGenerateInvites(t *testing.T) {
	t.Run("generates requested number of invites", func(t *testing.T) {
		db := database_mock.NewMockDB()
		logger := newTestLogger()
		auth := New(logger, db)

		invites, err := auth.GenerateInvites(context.Background(), 3)

		assert.NoError(t, err)
		assert.Len(t, invites, 3)
		for _, code := range invites {
			assert.NotEmpty(t, code)
		}
	})
}

func TestGetUsers(t *testing.T) {
	tests := []struct {
		name      string
		user      *entity.User
		setup     func(*database_mock.MockDB)
		wantErr   bool
		wantCount int
	}{
		{
			name:    "nil user denied",
			user:    nil,
			wantErr: true,
		},
		{
			name:    "non-admin denied",
			user:    &entity.User{Role: "user"},
			wantErr: true,
		},
		{
			name: "admin gets all users",
			user: &entity.User{Role: "admin"},
			setup: func(db *database_mock.MockDB) {
				db.SeedUser(&entity.User{Username: "user1", UserId: "id1"})
				db.SeedUser(&entity.User{Username: "user2", UserId: "id2"})
			},
			wantErr:   false,
			wantCount: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := database_mock.NewMockDB()
			logger := newTestLogger()
			auth := New(logger, db)

			if tt.setup != nil {
				tt.setup(db)
			}

			users, err := auth.GetUsers(context.Background(), tt.user)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.Len(t, users, tt.wantCount)
		})
	}
}

func TestGetUserById(t *testing.T) {
	tests := []struct {
		name         string
		userId       string
		setup        func(*database_mock.MockDB)
		wantErr      bool
		wantUsername string
		isNewUser    bool
	}{
		{
			name:    "empty user id",
			userId:  "",
			wantErr: true,
		},
		{
			name:   "existing user found",
			userId: "existing_user_id",
			setup: func(db *database_mock.MockDB) {
				db.SeedUser(&entity.User{
					Username: "existinguser",
					UserId:   "existing_user_id",
				})
			},
			wantErr:      false,
			wantUsername: "existinguser",
		},
		{
			name:      "new user auto-created",
			userId:    "new_user_id",
			setup:     nil,
			wantErr:   false,
			isNewUser: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := database_mock.NewMockDB()
			logger := newTestLogger()
			auth := New(logger, db)

			if tt.setup != nil {
				tt.setup(db)
			}

			user, err := auth.GetUserById(context.Background(), tt.userId)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.NotNil(t, user)

			if tt.isNewUser {
				assert.Contains(t, user.Username, "user_")
				assert.Equal(t, "App user", user.Name)
				assert.Equal(t, tt.userId, user.UserId)
			} else {
				assert.Equal(t, tt.wantUsername, user.Username)
			}
		})
	}
}
