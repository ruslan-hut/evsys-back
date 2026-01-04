package validate

import (
	"errors"
	"fmt"
	"regexp"
	"sync"

	"github.com/go-playground/validator/v10"
	"reflect"
	"time"
)

var (
	validate *validator.Validate
	once     sync.Once
)

// Valid user roles
var validUserRoles = map[string]bool{
	"":         true,
	"admin":    true,
	"operator": true,
}

// OCPP 1.6 ChargePointStatus values
var validConnectorStatuses = map[string]bool{
	"":              true,
	"Available":     true,
	"Preparing":     true,
	"Charging":      true,
	"SuspendedEV":   true,
	"SuspendedEVSE": true,
	"Finishing":     true,
	"Reserved":      true,
	"Unavailable":   true,
	"Faulted":       true,
}

// OCPP 1.6 StopTransaction reason values
var validStopReasons = map[string]bool{
	"":               true,
	"EmergencyStop":  true,
	"EVDisconnected": true,
	"HardReset":      true,
	"Local":          true,
	"Other":          true,
	"PowerLoss":      true,
	"Reboot":         true,
	"Remote":         true,
	"SoftReset":      true,
	"UnlockCommand":  true,
	"DeAuthorized":   true,
}

// WebSocket command names
var validWsCommands = map[string]bool{
	"StartTransaction":      true,
	"StopTransaction":       true,
	"CheckStatus":           true,
	"ListenTransaction":     true,
	"StopListenTransaction": true,
	"ListenChargePoints":    true,
	"ListenLog":             true,
	"PingConnection":        true,
}

// Email regex pattern (RFC 5322 simplified)
var emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)

func initValidator() {
	validate = validator.New()

	// Register custom validators
	_ = validate.RegisterValidation("email_rfc", validateEmailRFC)
	_ = validate.RegisterValidation("user_role", validateUserRole)
	_ = validate.RegisterValidation("connector_status", validateConnectorStatus)
	_ = validate.RegisterValidation("stop_reason", validateStopReason)
	_ = validate.RegisterValidation("ws_command", validateWsCommand)
}

// validateEmailRFC validates email format, allows empty string
func validateEmailRFC(fl validator.FieldLevel) bool {
	email := fl.Field().String()
	if email == "" {
		return true
	}
	return emailRegex.MatchString(email)
}

// validateUserRole validates user role against allowed values
func validateUserRole(fl validator.FieldLevel) bool {
	return validUserRoles[fl.Field().String()]
}

// validateConnectorStatus validates OCPP connector status
func validateConnectorStatus(fl validator.FieldLevel) bool {
	return validConnectorStatuses[fl.Field().String()]
}

// validateStopReason validates OCPP stop transaction reason
func validateStopReason(fl validator.FieldLevel) bool {
	return validStopReasons[fl.Field().String()]
}

// validateWsCommand validates WebSocket command names
func validateWsCommand(fl validator.FieldLevel) bool {
	return validWsCommands[fl.Field().String()]
}

// Struct validates a single struct object
func Struct(s interface{}) error {
	if s == nil {
		return fmt.Errorf("is nil")
	}
	if !isStruct(s) {
		return fmt.Errorf("not a struct")
	}

	once.Do(initValidator)

	var validationErrors validator.ValidationErrors
	var invalidValidationError *validator.InvalidValidationError

	err := validate.Struct(s)
	if err == nil {
		return nil
	}

	if errors.As(err, &validationErrors) {
		message := ""
		for _, fieldErr := range validationErrors {
			if len(message) > 0 {
				message += "; "
			}
			message += fmt.Sprintf("%s %s", fieldErr.Field(), fieldErr.Tag())
		}
		return errors.New(message)
	} else if errors.As(err, &invalidValidationError) {
		return fmt.Errorf("invalid validation error: %w", err)
	} else {
		return fmt.Errorf("unknown validation error: %w", err)
	}
}

func isStruct(s interface{}) bool {
	r := reflect.TypeOf(s)
	if r.Kind() == reflect.Ptr {
		r = r.Elem()
	}
	return r.Kind() == reflect.Struct
}

// DateString validates a date string in RFC3339 format
// Returns the parsed date or an error
func DateString(s string) (time.Time, error) {
	if s == "" {
		return time.Now(), nil
	}
	date, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Now(), fmt.Errorf("%s: %w", s, err)
	}
	return date, nil
}
