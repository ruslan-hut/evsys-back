# Project Review: evsys-back

## Executive Summary

This is a well-structured Go backend for EV charging station management. The codebase demonstrates solid architectural patterns with recent improvements to connection management and reliability.

**Overall Assessment**: Good foundation, production-ready structure. Performance issues resolved, needs testing.

**Recent Improvements** (December 2024):
- MongoDB connection pooling implemented
- HTTP client timeouts added
- Graceful shutdown with proper cleanup

---

## Architecture Analysis

### Strengths

#### 1. Clean Layered Architecture
The project follows a clean separation of concerns:
```
main.go → config → implementations → handlers → entities
```

Each layer has clear responsibilities:
- `config/` - Configuration loading
- `impl/` - Business logic implementations
- `internal/api/` - HTTP/WebSocket handling
- `entity/` - Domain models

#### 2. Repository Pattern with Dependency Injection
The `impl/core/repository.go` defines clean interfaces that both MongoDB and mock implementations satisfy. This enables:
- Easy testing with mock implementations
- Swappable storage backends
- Loose coupling between business logic and persistence

```go
// Good: Interface-based design
type Repository interface {
    GetChargePoints(level int, searchTerm string) ([]*entity.ChargePoint, error)
    // ...
}
```

#### 3. Interface Composition in Server
The `internal/api/http/server.go` composes multiple handler interfaces into a single `Core` interface:
```go
type Core interface {
    helper.Helper
    authenticate.Authenticate
    users.Users
    locations.Locations
    // ...
}
```
This provides compile-time verification that the core implementation satisfies all handler requirements.

#### 4. Structured Logging
Consistent use of `log/slog` with module attribution (`sl.Module()`) and secret masking (`sl.Secret()`). Request tracking via Chi middleware.

#### 5. Handler Function Pattern
Handlers are implemented as factory functions returning `http.HandlerFunc`:
```go
func Authenticate(logger *slog.Logger, handler Users) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) { ... }
}
```
This pattern enables clean dependency injection and testability.

---

### Critical Issues

#### 1. No Test Coverage
**Severity: Critical**

The project has zero test files. For production code handling payments and charging transactions, this is a significant risk.

**Recommendation**: Prioritize adding tests for:
- Authentication flow (`impl/authenticator/`)
- Payment operations (`impl/core/core.go:271-308`)
- Transaction state management
- WebSocket message handling

#### 2. ~~MongoDB Connection Per Operation~~ FIXED
**Severity: High - Performance** | **Status: Resolved**

~~Every database operation creates a new connection and disconnects after.~~

**Fix Applied**: Refactored to use a single persistent `*mongo.Client` initialized at startup. The MongoDB Go driver now handles connection pooling internally. Changes:
- `NewMongoClient()` connects once and stores the client
- Added `Close()` method for graceful shutdown
- Removed `connect()`/`disconnect()` from all 48 methods
- Connection verified with `Ping()` at startup

#### 3. Global Context in MongoDB Client
**Severity: Medium**

The MongoDB client stores `context.Background()` as a field and uses it for all operations:
```go
// impl/database/mongo.go:81-86
client := &MongoDB{
    ctx: context.Background(),  // Never expires
    // ...
}
```

This prevents proper request cancellation and timeout propagation from HTTP handlers.

**Recommendation**: Accept context as parameter in each method.

---

### Moderate Issues

#### 4. Mutex Overuse in Authenticator
**Severity: Medium**

The authenticator uses a single mutex for all operations:
```go
// impl/authenticator/authenticator.go
func (a *Authenticator) AuthenticateByToken(token string) (*entity.User, error) {
    a.mux.Lock()
    defer a.mux.Unlock()
    // ...
}
```

Since database operations are already thread-safe, this creates unnecessary serialization of authentication requests.

#### 5. Error Handling Inconsistency
**Severity: Low**

Some methods return `nil, nil` on not-found (using `findError`), while others return errors. This inconsistency can cause nil pointer issues:
```go
// Returns nil, nil on not found
func (m *MongoDB) findError(err error) error {
    if errors.Is(err, mongo.ErrNoDocuments) {
        return nil
    }
    return err
}
```

#### 6. WebSocket Origin Check Disabled
**Severity: Medium - Security**

```go
// internal/api/http/server.go:72-75
upgrader: websocket.Upgrader{
    CheckOrigin: func(r *http.Request) bool {
        return true  // Allows all origins
    },
},
```

In production, this should validate the Origin header against allowed domains.

#### 7. Hardcoded Test Token in Mock DB
**Severity: Low**

```go
// impl/database-mock/mock-db.go:56
if token == "12345678901234567890123456789000" {
```

Should be configurable or removed in production builds.

#### 8. ~~Missing HTTP Client Timeout~~ FIXED
**Severity: Medium** | **Status: Resolved**

~~The central system client uses default HTTP client with no timeout.~~

**Fix Applied**: Added reusable HTTP client with 30-second timeout to `CentralSystem` struct:
```go
client: &http.Client{
    Timeout: 30 * time.Second,
}
```

---

## Code Quality Observations

### Positive Patterns

1. **Consistent Package Structure**: Each handler package defines its own interface slice of requirements
2. **Chi Router Usage**: Clean route grouping with middleware
3. **Password Hashing**: Uses bcrypt for password storage
4. **Validation**: Uses `go-playground/validator` for struct validation
5. **Secret Masking in Logs**: Sensitive data is truncated in log output

### Areas for Improvement

1. **Magic Numbers**: Several hardcoded values should be constants
   - Token length (32)
   - Timeout durations (90s, 5s)
   - Max access level (10)

2. **Comments**: Core business logic lacks documentation
   - `NormalizeMeterValues` function not documented
   - Access level checks not explained

3. **Error Codes**: Consistent use of error code 2001 for all errors:
   ```go
   response.Error(2001, message)  // Same code everywhere
   ```

---

## WebSocket Implementation Review

### Strengths
- Proper goroutine lifecycle management with `writePump` and `readPump`
- Connection pool pattern with channels
- State restoration on reconnect (`restoreUserState`)
- Multiple subscription types (broadcast, log-event, charge-point-event)

### Concerns
- Long polling intervals for transaction state (2-5 seconds)
- No ping/pong heartbeat mechanism for connection health
- `time.Sleep(1 * time.Second)` in `listenForTransactionState` can cause message delays

---

## Security Review

### Good Practices
1. Password hashing with bcrypt
2. Token-based authentication with configurable length
3. Role-based access control (admin, operator, user)
4. Access level checks on charge points and commands
5. Secret masking in logs

### Concerns
1. WebSocket allows all origins
2. No rate limiting on authentication endpoints
3. Firebase token validation errors expose internal details
4. Invite codes are deleted after use (good), but no expiration

---

## Recommendations Priority

### Immediate (Week 1)
1. ~~Fix MongoDB connection management - use single client~~ DONE
2. ~~Add timeout to HTTP client in central-system~~ DONE
3. ~~Add graceful shutdown for MongoDB connections~~ DONE
4. Pass context through database methods

### Short-term (Month 1)
1. Add unit tests for authentication and payment flows
2. Add integration tests for API endpoints
3. Configure WebSocket origin validation for production

### Medium-term (Quarter 1)
1. Add rate limiting to public endpoints
2. Implement proper error codes
3. Add metrics/observability
4. Document access level and role system

---

## File Structure Summary

```
evsys-back/
├── main.go                          # Entry point, DI setup
├── config/config.go                 # YAML config loader
├── entity/                          # 24 domain models
├── impl/
│   ├── core/core.go                 # Business logic (370 lines)
│   ├── core/repository.go           # Repository interface
│   ├── database/mongo.go            # MongoDB (1359 lines) ← Largest file
│   ├── database-mock/mock-db.go     # Stub implementation
│   ├── authenticator/               # Auth logic
│   ├── reports/                     # Statistics
│   ├── central-system/              # External API
│   └── status-reader/               # Transaction state
├── internal/
│   ├── api/http/server.go           # Server + WebSocket (730 lines)
│   ├── api/handlers/                # REST endpoints
│   ├── api/middleware/              # authenticate, timeout
│   └── lib/                         # Utilities
└── .github/workflows/deploy.yml     # CI/CD
```

**Lines of Code**: ~5,500 (Go source files)
**Test Coverage**: 0%
**Entity Models**: 24 files

---

## Conclusion

The evsys-back project has a solid architectural foundation with proper separation of concerns and good use of Go idioms. The main concerns are:

1. **Zero test coverage** - High risk for production system handling payments
2. ~~**MongoDB connection management** - Performance bottleneck~~ FIXED
3. **Context propagation** - Limits proper timeout/cancellation

The WebSocket implementation is well-designed for real-time updates. The authentication and authorization model is reasonable for the domain.

### Recent Fixes Applied

| Issue | Status | Impact |
|-------|--------|--------|
| MongoDB connection per operation | Fixed | Major performance improvement - connection pooling now active |
| HTTP client timeout | Fixed | Prevents indefinite hangs on external API calls |
| Graceful shutdown | Added | Clean MongoDB disconnection on SIGTERM/SIGINT |

With these fixes applied, the remaining priorities are test coverage and context propagation.
