package internal

import (
	"crypto/rand"
	"encoding/hex"
	"evsys-back/models"
	"evsys-back/services"
	"fmt"
	"golang.org/x/crypto/bcrypt"
	"strings"
	"sync"
	"time"
)

const tokenLength = 32

type Authenticator struct {
	logger   services.LogHandler
	database services.Database
	firebase services.FirebaseAuth
	mux      *sync.Mutex
}

func NewAuthenticator() *Authenticator {
	return &Authenticator{
		mux: &sync.Mutex{},
	}
}

func (a *Authenticator) SetLogger(logger services.LogHandler) {
	a.logger = logger
}

func (a *Authenticator) SetDatabase(database services.Database) {
	a.database = database
}

func (a *Authenticator) SetFirebase(firebase services.FirebaseAuth) {
	a.firebase = firebase
}

func (a *Authenticator) GetUser(token string) (*models.User, error) {
	if token == "" {
		return nil, fmt.Errorf("empty token")
	}
	if a.database == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	a.mux.Lock()
	defer a.mux.Unlock()
	if len(token) < tokenLength {
		return nil, fmt.Errorf("invalid token")
	}
	var user *models.User
	// considering token is database token
	if len(token) == tokenLength {
		user, _ = a.database.CheckToken(token)
		if user == nil {
			return nil, fmt.Errorf("token check failed")
		}
		_ = a.updateLastSeen(user)
		return user, nil
	}
	// considering token is firebase token
	if a.firebase != nil {
		userId, err := a.firebase.CheckToken(token)
		if err != nil {
			return nil, fmt.Errorf("firebase token check: %s", err)
		}
		if userId != "" {
			user, _ = a.database.GetUserById(userId)
			if user == nil {
				username := fmt.Sprintf("user_%s", a.generateKey(5))
				user = &models.User{
					Username:       username,
					Name:           "Firebase user",
					UserId:         userId,
					DateRegistered: time.Now(),
				}
				err = a.database.AddUser(user)
				if err != nil {
					return nil, fmt.Errorf("adding firebase user: %s", err)
				}
			}
			_ = a.updateLastSeen(user)
			return user, nil
		}
	}
	return nil, fmt.Errorf("token check failed")
}

// update user last seen
func (a *Authenticator) updateLastSeen(user *models.User) error {
	err := a.database.UpdateLastSeen(user)
	if err != nil {
		return fmt.Errorf("updating user last seen: %s", err)
	}
	return nil
}

func (a *Authenticator) GenerateInvites(count int) ([]string, error) {
	if a.database == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	a.mux.Lock()
	defer a.mux.Unlock()
	invites := make([]string, 0)
	for i := 0; i < count; i++ {
		inviteCode := a.generateKey(5)
		invite := &models.Invite{
			Code: inviteCode,
		}
		err := a.database.AddInviteCode(invite)
		if err != nil {
			a.logger.Error("adding invite code: %s", err)
			continue
		}
		invites = append(invites, inviteCode)
	}
	if len(invites) == 0 {
		return nil, fmt.Errorf("no invites generated")
	}
	return invites, nil
}

func (a *Authenticator) GetUserTag(user *models.User) (string, error) {
	if a.database == nil {
		return "", fmt.Errorf("database not initialized")
	}
	a.mux.Lock()
	defer a.mux.Unlock()
	tags, _ := a.database.GetUserTags(user.UserId)
	if tags == nil {
		tags = make([]models.UserTag, 0)
	}
	// auto create new tag in enabled state, if no tags found
	// because user is authenticated by token, not by tag
	if len(tags) == 0 {
		newIdTag := strings.ToUpper(a.generateKey(20))
		newTag := models.UserTag{
			UserId:         user.UserId,
			Username:       user.Username,
			IdTag:          newIdTag,
			IsEnabled:      true,
			Note:           "api: added by request",
			DateRegistered: time.Now(),
		}
		err := a.database.AddUserTag(&newTag)
		if err != nil {
			return "", fmt.Errorf("adding user tag: %s", err)
		}
		tags = append(tags, newTag)
	}
	_ = a.database.UpdateTagLastSeen(&tags[0])
	return tags[0].IdTag, nil
}

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

// AuthenticateUser method returns user with blank password if authentication is successful
// Note: in a database, the password should be stored as a hash
func (a *Authenticator) AuthenticateUser(username, password string) (*models.User, error) {
	if a.database == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	a.mux.Lock()
	defer a.mux.Unlock()
	user, err := a.database.GetUser(username)
	if err != nil {
		return nil, err
	}
	err = bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password))
	if err != nil {
		return nil, err

	}
	token := a.generateKey(tokenLength)
	user.Token = token
	user.LastSeen = time.Now()
	err = a.database.UpdateLastSeen(user)
	if err != nil {
		return nil, err
	}
	user.Password = ""
	a.logger.Info(fmt.Sprintf("user %s logged in with password", user.Username))
	return user, nil
}

// RegisterUser add new user to the database
// for new user registration, using "token" field as a value of invite code
func (a *Authenticator) RegisterUser(user *models.User) error {
	if a.database == nil {
		return fmt.Errorf("database not initialized")
	}
	if user.Password == "" {
		return fmt.Errorf("empty password")
	}
	if user.Username == "" {
		return fmt.Errorf("empty username")
	}
	if user.Token == "" {
		return fmt.Errorf("empty invite code")
	}
	a.mux.Lock()
	defer a.mux.Unlock()
	existedUser, _ := a.database.GetUser(user.Username)
	if existedUser != nil {
		return fmt.Errorf("user already exists")
	}
	ok, _ := a.database.CheckInviteCode(user.Token)
	if !ok {
		return fmt.Errorf("invalid invite code")
	}
	user.Password = a.generatePasswordHash(user.Password)
	if user.Password == "" {
		return fmt.Errorf("empty password hash")
	}
	user.DateRegistered = time.Now()
	err := a.database.AddUser(user)
	if err != nil {
		return err
	}
	err = a.database.DeleteInviteCode(user.Token)
	if err != nil {
		a.logger.Error("deleting invite code", err)
	}
	return nil
}

func (a *Authenticator) generatePasswordHash(password string) string {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		a.logger.Error("generating password hash", err)
		return ""
	}
	return string(hash)
}
