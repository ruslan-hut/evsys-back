package authenticator

import (
	"context"
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
	defaultUserGroupId = "default"
)

// commands allowed only for admins
var adminCommands = []string{"ChangeConfiguration", "SetChargingProfile", "ClearChargingProfile", "GetDiagnostics"}

// commands allowed for all users
var userCommands = []string{"RemoteStartTransaction", "RemoteStopTransaction"}

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

func (a *Authenticator) AuthenticateByToken(ctx context.Context, token string) (*entity.User, error) {
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
		user, _ = a.database.CheckToken(ctx, token)
		if user == nil {
			return nil, fmt.Errorf("token check failed")
		}
		_ = a.updateLastSeen(ctx, user)
		return user, nil
	}
	// considering token is firebase token
	if a.firebase != nil {
		userId, err := a.firebase.CheckToken(token)
		if err != nil {
			return nil, fmt.Errorf("firebase token check: %s", err)
		}
		user, err = a.GetUserById(ctx, userId)
		if err != nil {
			return nil, fmt.Errorf("getting user by id: %s", err)
		}
		// put token to user data, frontend uses it for further requests
		user.Token = token
		return user, nil
	}
	return nil, fmt.Errorf("token check failed")
}

func (a *Authenticator) GetUserById(ctx context.Context, userId string) (*entity.User, error) {
	if userId == "" {
		return nil, fmt.Errorf("empty user id")
	}

	user, _ := a.database.GetUserById(ctx, userId)

	if user == nil {

		// generating unique username for new user
		nameLength := 5
		username := fmt.Sprintf("user_%s", a.generateKey(nameLength))
		for a.database.CheckUsername(ctx, username) != nil {
			nameLength++
			username = fmt.Sprintf("user_%s", a.generateKey(nameLength))
		}

		user = &entity.User{
			Username:       username,
			Name:           "App user",
			UserId:         userId,
			PaymentPlan:    defaultPaymentPlan,
			Group:          defaultUserGroupId,
			DateRegistered: time.Now(),
		}

		err := a.database.AddUser(ctx, user)
		if err != nil {
			return nil, fmt.Errorf("adding user: %s", err)
		}
		a.logger.With(
			slog.String("username", user.Username),
			sl.Secret("user_id", userId),
		).Info("new user registered")
	}

	_ = a.updateLastSeen(ctx, user)
	return user, nil
}

func (a *Authenticator) GenerateInvites(ctx context.Context, count int) ([]string, error) {
	a.mux.Lock()
	defer a.mux.Unlock()
	invites := make([]string, 0)
	for i := 0; i < count; i++ {
		inviteCode := a.generateKey(5)
		invite := &entity.Invite{
			Code: inviteCode,
		}
		err := a.database.AddInviteCode(ctx, invite)
		if err != nil {
			a.logger.Error("add invite code", err)
			continue
		}
		invites = append(invites, inviteCode)
	}
	if len(invites) == 0 {
		return nil, fmt.Errorf("no invites generated")
	}
	return invites, nil
}

func (a *Authenticator) GetUserTag(ctx context.Context, user *entity.User) (string, error) {
	a.mux.Lock()
	defer a.mux.Unlock()
	tags, _ := a.database.GetUserTags(ctx, user.UserId)
	if tags == nil {
		tags = make([]entity.UserTag, 0)
	}
	// auto create new tag in enabled state, if no tags found
	// because user is authenticated by token, not by tag
	if len(tags) == 0 {

		newIdTag := strings.ToUpper(a.generateKey(20))
		for a.database.CheckUserTag(ctx, newIdTag) != nil {
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

		err := a.database.AddUserTag(ctx, &newTag)
		if err != nil {
			return "", fmt.Errorf("adding user tag: %s", err)
		}
		tags = append(tags, newTag)
	}
	_ = a.database.UpdateTagLastSeen(ctx, &tags[0])
	return tags[0].IdTag, nil
}

// AuthenticateUser method returns user with blank password if authentication is successful
// Note: in a database, the password should be stored as a hash
func (a *Authenticator) AuthenticateUser(ctx context.Context, username, password string) (*entity.User, error) {
	a.mux.Lock()
	defer a.mux.Unlock()
	user, err := a.database.GetUser(ctx, username)
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
	err = a.database.UpdateLastSeen(ctx, user)
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
func (a *Authenticator) RegisterUser(ctx context.Context, user *entity.User) error {
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
	existedUser, _ := a.database.GetUser(ctx, user.Username)
	if existedUser != nil {
		return fmt.Errorf("user already exists")
	}
	// check provided invite code
	ok, _ := a.database.CheckInviteCode(ctx, user.Token)
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
	// assign default user group
	if user.Group == "" {
		user.Group = defaultUserGroupId
	}
	// generate unique user id
	user.UserId = a.getUserId(ctx)
	user.DateRegistered = time.Now()
	// store new user in repository
	err := a.database.AddUser(ctx, user)
	if err != nil {
		return err
	}
	// delete used invite code
	err = a.database.DeleteInviteCode(ctx, user.Token)
	if err != nil {
		a.logger.Error("deleting invite code", err)
	}
	return nil
}

func (a *Authenticator) GetUsers(ctx context.Context, user *entity.User) ([]*entity.User, error) {
	if user == nil || !user.IsAdmin() {
		return nil, fmt.Errorf("access denied")
	}
	a.mux.Lock()
	defer a.mux.Unlock()
	users, err := a.database.GetUsers(ctx)
	if err != nil {
		return nil, err
	}
	return users, nil
}

// CommandAccess check user access for a specific command based on role and permissions
func (a *Authenticator) CommandAccess(user *entity.User, command string) error {
	if user == nil {
		return fmt.Errorf("user undefined")
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

// HasAccess check access permission for a user to a specific subsystem based on user type
func (a *Authenticator) HasAccess(user *entity.User, subSystem string) error {
	if user == nil {
		return fmt.Errorf("user undefined")
	}
	if user.IsPowerUser() {
		return nil
	}
	return fmt.Errorf("access to %s denied", subSystem)
}

// CreateUser creates a new user (admin operation, no invite code required)
func (a *Authenticator) CreateUser(ctx context.Context, user *entity.User) error {
	if user.Password == "" {
		return fmt.Errorf("password is required")
	}
	if user.Username == "" {
		return fmt.Errorf("username is required")
	}
	if len(user.Username) < 3 {
		return fmt.Errorf("username must be at least 3 characters")
	}
	if len(user.Password) < 6 {
		return fmt.Errorf("password must be at least 6 characters")
	}
	a.mux.Lock()
	defer a.mux.Unlock()

	// check if username exists
	existedUser, _ := a.database.GetUser(ctx, user.Username)
	if existedUser != nil {
		return fmt.Errorf("username already exists")
	}

	// hash the password
	user.Password = a.generatePasswordHash(user.Password)
	if user.Password == "" {
		return fmt.Errorf("failed to hash password")
	}

	// assign defaults
	if user.PaymentPlan == "" {
		user.PaymentPlan = defaultPaymentPlan
	}
	if user.Group == "" {
		user.Group = defaultUserGroupId
	}
	if user.AccessLevel < 0 || user.AccessLevel > 10 {
		user.AccessLevel = 0
	}

	// generate unique user id
	user.UserId = a.getUserId(ctx)
	user.DateRegistered = time.Now()

	// generate token for the new user
	user.Token = a.generateKey(tokenLength)

	err := a.database.AddUser(ctx, user)
	if err != nil {
		return fmt.Errorf("failed to create user: %w", err)
	}

	a.logger.With(
		slog.String("username", user.Username),
		sl.Secret("user_id", user.UserId),
	).Info("user created by admin")

	return nil
}

// UpdateUser updates an existing user's information
func (a *Authenticator) UpdateUser(ctx context.Context, username string, updates *entity.User) (*entity.User, error) {
	a.mux.Lock()
	defer a.mux.Unlock()

	// get existing user
	user, err := a.database.GetUser(ctx, username)
	if err != nil || user == nil {
		return nil, fmt.Errorf("user not found")
	}

	// apply updates (only non-empty fields)
	if updates.Name != "" {
		user.Name = updates.Name
	}
	if updates.Email != "" {
		user.Email = updates.Email
	}
	if updates.Password != "" {
		if len(updates.Password) < 6 {
			return nil, fmt.Errorf("password must be at least 6 characters")
		}
		user.Password = a.generatePasswordHash(updates.Password)
		if user.Password == "" {
			return nil, fmt.Errorf("failed to hash password")
		}
	}
	if updates.Role != "" || updates.Role == "" && user.Role != "" {
		// allow setting role to empty (regular user)
		user.Role = updates.Role
	}
	if updates.AccessLevel >= 0 && updates.AccessLevel <= 10 {
		user.AccessLevel = updates.AccessLevel
	}
	if updates.PaymentPlan != "" {
		user.PaymentPlan = updates.PaymentPlan
	}

	err = a.database.UpdateUser(ctx, user)
	if err != nil {
		return nil, fmt.Errorf("failed to update user: %w", err)
	}

	a.logger.With(
		slog.String("username", user.Username),
	).Info("user updated")

	return user, nil
}

// DeleteUser deletes a user from the system
func (a *Authenticator) DeleteUser(ctx context.Context, username string) error {
	a.mux.Lock()
	defer a.mux.Unlock()

	// check if user exists
	user, err := a.database.GetUser(ctx, username)
	if err != nil || user == nil {
		return fmt.Errorf("user not found")
	}

	err = a.database.DeleteUser(ctx, username)
	if err != nil {
		return fmt.Errorf("failed to delete user: %w", err)
	}

	a.logger.With(
		slog.String("username", username),
	).Info("user deleted")

	return nil
}
