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
)

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
	user, _ := a.database.CheckToken(token)
	if user != nil {
		return user, nil
	}
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
					Username: username,
					Name:     "Firebase user",
					UserId:   userId,
				}
				err = a.database.AddUser(user)
				if err != nil {
					return nil, fmt.Errorf("adding firebase user: %s", err)
				}
			}
			return user, nil
		}
	}
	return nil, fmt.Errorf("token check failed")
}

func (a *Authenticator) GetUserTag(userId string) (string, error) {
	if a.database == nil {
		return "", fmt.Errorf("database not initialized")
	}
	a.mux.Lock()
	defer a.mux.Unlock()
	tags, _ := a.database.GetUserTags(userId)
	if tags == nil {
		tags = make([]models.UserTag, 0)
	}
	if len(tags) == 0 {
		newIdTag := strings.ToUpper(a.generateKey(20))
		newTag := models.UserTag{
			UserId:    userId,
			Username:  "",
			IdTag:     newIdTag,
			IsEnabled: true,
		}
		err := a.database.AddUserTag(&newTag)
		if err != nil {
			return "", fmt.Errorf("adding user tag: %s", err)
		}
		tags = append(tags, newTag)
	}
	return tags[0].IdTag, nil
}

func (a *Authenticator) generateKey(length int) string {
	b := make([]byte, 20)
	_, err := rand.Read(b)
	if err != nil {
		return ""
	}
	s := hex.EncodeToString(b)
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
	result := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password))
	if result == nil {
		token := a.generateKey(32)
		user.Token = token
		err = a.database.UpdateUser(user)
		if err != nil {
			return nil, err
		}
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
