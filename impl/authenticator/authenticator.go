package authenticator

import (
	"crypto/rand"
	"encoding/hex"
	"evsys-back/entity"
	"evsys-back/internal/lib/sl"
	"fmt"
	"golang.org/x/crypto/bcrypt"
	"log/slog"
	"strings"
	"sync"
	"time"
)

const (
	tokenLength        = 32
	defaultPaymentPlan = "default"
	defaultUserRole    = "user"
)

// commands allowed only for admins
var adminCommands = []string{"ChangeConfiguration", "SetChargingProfile", "ClearChargingProfile", "GetDiagnostics"}

// commands allowed for all users
var userCommands = []string{"RemoteStartTransaction", "RemoteStopTransaction"}

type Repository interface {
	GetUser(username string) (*entity.User, error)
	GetUserById(userId string) (*entity.User, error)
	UpdateLastSeen(user *entity.User) error
	AddUser(user *entity.User) error
	CheckUsername(username string) error
	CheckToken(token string) (*entity.User, error)
	AddInviteCode(invite *entity.Invite) error
	CheckInviteCode(code string) (bool, error)
	DeleteInviteCode(code string) error
	GetUserTags(userId string) ([]entity.UserTag, error)
	AddUserTag(userTag *entity.UserTag) error
	CheckUserTag(idTag string) error
	UpdateTagLastSeen(userTag *entity.UserTag) error
	GetUsers() ([]*entity.User, error)
}

type FirebaseAuth interface {
	CheckToken(token string) (string, error)
}

type Authenticator struct {
	logger   *slog.Logger
	database Repository
	firebase FirebaseAuth
	mux      sync.Mutex
}

func New(log *slog.Logger, repo Repository) *Authenticator {
	if repo == nil {
		log.Error("authenticator init failed: nil repository")
		return nil
	}
	return &Authenticator{
		database: repo,
		logger:   log.With(sl.Module("impl.authenticator")),
		mux:      sync.Mutex{},
	}
}

func (a *Authenticator) SetFirebase(firebase FirebaseAuth) {
	a.firebase = firebase
}

func (a *Authenticator) AuthenticateByToken(token string) (*entity.User, error) {
	if token == "" {
		return nil, fmt.Errorf("empty token")
	}
	a.mux.Lock()
	defer a.mux.Unlock()
	if len(token) < tokenLength {
		return nil, fmt.Errorf("invalid token")
	}
	var user *entity.User
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
		user, err = a.GetUserById(userId)
		if err != nil {
			return nil, fmt.Errorf("getting user by id: %s", err)
		}
		// put token to user data, frontend uses it for further requests
		user.Token = token
		return user, nil
	}
	return nil, fmt.Errorf("token check failed")
}

func (a *Authenticator) GetUserById(userId string) (*entity.User, error) {
	if userId == "" {
		return nil, fmt.Errorf("empty user id")
	}

	user, _ := a.database.GetUserById(userId)

	if user == nil {

		// generating unique username for new user
		nameLength := 5
		username := fmt.Sprintf("user_%s", a.generateKey(nameLength))
		for a.database.CheckUsername(username) != nil {
			nameLength++
			username = fmt.Sprintf("user_%s", a.generateKey(nameLength))
		}

		user = &entity.User{
			Username:       username,
			Name:           "Firebase user",
			UserId:         userId,
			PaymentPlan:    defaultPaymentPlan,
			DateRegistered: time.Now(),
		}

		err := a.database.AddUser(user)
		if err != nil {
			return nil, fmt.Errorf("adding user: %s", err)
		}
		a.logger.With(
			slog.String("username", user.Username),
			sl.Secret("user_id", userId),
		).Info("new user registered")
	}

	_ = a.updateLastSeen(user)
	return user, nil
}

// update user last seen
func (a *Authenticator) updateLastSeen(user *entity.User) error {
	err := a.database.UpdateLastSeen(user)
	if err != nil {
		return fmt.Errorf("updating user last seen: %s", err)
	}
	return nil
}

func (a *Authenticator) GenerateInvites(count int) ([]string, error) {
	a.mux.Lock()
	defer a.mux.Unlock()
	invites := make([]string, 0)
	for i := 0; i < count; i++ {
		inviteCode := a.generateKey(5)
		invite := &entity.Invite{
			Code: inviteCode,
		}
		err := a.database.AddInviteCode(invite)
		if err != nil {
			a.logger.Error("add invite code", sl.Err(err))
			continue
		}
		invites = append(invites, inviteCode)
	}
	if len(invites) == 0 {
		return nil, fmt.Errorf("no invites generated")
	}
	return invites, nil
}

func (a *Authenticator) GetUserTag(user *entity.User) (string, error) {
	a.mux.Lock()
	defer a.mux.Unlock()
	tags, _ := a.database.GetUserTags(user.UserId)
	if tags == nil {
		tags = make([]entity.UserTag, 0)
	}
	// auto create new tag in enabled state, if no tags found
	// because user is authenticated by token, not by tag
	if len(tags) == 0 {

		newIdTag := strings.ToUpper(a.generateKey(20))
		for a.database.CheckUserTag(newIdTag) != nil {
			newIdTag = strings.ToUpper(a.generateKey(20))
		}

		newTag := entity.UserTag{
			UserId:         user.UserId,
			Username:       user.Username,
			IdTag:          newIdTag,
			IsEnabled:      true,
			Source:         "AP",
			Note:           "",
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

// AuthenticateUser method returns user with blank password if authentication is successful
// Note: in a database, the password should be stored as a hash
func (a *Authenticator) AuthenticateUser(username, password string) (*entity.User, error) {
	a.mux.Lock()
	defer a.mux.Unlock()
	user, err := a.database.GetUser(username)
	if err != nil {
		return nil, fmt.Errorf("user not found: %s", username)
	}
	err = bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password))
	if err != nil {
		return nil, fmt.Errorf("password check failed: %s", username)
	}
	token := a.generateKey(tokenLength)
	user.Token = token
	user.LastSeen = time.Now()
	err = a.database.UpdateLastSeen(user)
	if err != nil {
		return nil, err
	}
	user.Password = ""
	a.logger.With(
		slog.String("username", user.Username),
		sl.Secret("user_id", user.UserId),
		sl.Secret("token", token),
	).Info("user authenticated")
	return user, nil
}

// RegisterUser registers a new user in the system with validation checks and default assignments.
func (a *Authenticator) RegisterUser(user *entity.User) error {
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
	// check if username exists
	existedUser, _ := a.database.GetUser(user.Username)
	if existedUser != nil {
		return fmt.Errorf("user already exists")
	}
	// check provided invite code
	ok, _ := a.database.CheckInviteCode(user.Token)
	if !ok {
		return fmt.Errorf("invalid invite code")
	}
	// get password hash to store in user's data
	user.Password = a.generatePasswordHash(user.Password)
	if user.Password == "" {
		return fmt.Errorf("empty password hash")
	}
	// assign default payment plan
	if user.PaymentPlan == "" {
		user.PaymentPlan = defaultPaymentPlan
	}
	// assign default user role
	if user.Role == "" {
		user.Role = defaultUserRole
	}
	// generate unique user id
	user.UserId = a.getUserId()
	user.DateRegistered = time.Now()
	// store new user in repository
	err := a.database.AddUser(user)
	if err != nil {
		return err
	}
	// delete used invite code
	err = a.database.DeleteInviteCode(user.Token)
	if err != nil {
		a.logger.Error("deleting invite code", sl.Err(err))
	}
	return nil
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

func (a *Authenticator) GetUsers(user *entity.User) ([]*entity.User, error) {
	if user == nil || !user.IsAdmin() {
		return nil, fmt.Errorf("access denied")
	}
	a.mux.Lock()
	defer a.mux.Unlock()
	users, err := a.database.GetUsers()
	if err != nil {
		return nil, err
	}
	return users, nil
}

func (a *Authenticator) CommandAccess(user *entity.User, command string) error {
	if user == nil {
		return fmt.Errorf("access denied")
	}
	if user.IsAdmin() {
		return nil
	}
	for _, c := range userCommands {
		if c == command {
			return nil
		}
	}
	if !user.IsPowerUser() {
		return fmt.Errorf("access denied")
	}
	for _, c := range adminCommands {
		if c == command {
			return fmt.Errorf("access denied")
		}
	}
	return nil
}

func (a *Authenticator) generatePasswordHash(password string) string {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		a.logger.Error("generating password hash", sl.Err(err))
		return ""
	}
	return string(hash)
}
