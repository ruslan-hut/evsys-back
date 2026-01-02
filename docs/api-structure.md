# API Structure

This document describes the overall architecture and structure of the evsys-back REST API and WebSocket interface.

**Related documentation:** [API Endpoints Reference](api-endpoints.md)

## Overview

The API provides two communication channels:
- **REST API** at `/api/v1` for synchronous request-response operations
- **WebSocket** at `/ws` for real-time bidirectional communication

## Base URL

```
REST:      /api/v1
WebSocket: /ws
```

## Authentication

### Token-Based Authentication

Most REST endpoints require authentication via the `Authorization` header:

```
Authorization: Bearer <token>
```

The token is obtained by calling the [authenticate endpoint](api-endpoints.md#post-apiv1usersauthenticate) with valid credentials.

### Access Levels

Users have access levels (0-10) that determine which resources they can access:
- **Level 0**: Basic user access
- **Higher levels**: Access to more locations and charge points

### Roles

- `admin`: Full system access
- `operator`: Extended access for operators
- `user`: Standard user access

## Response Format

### Success Response

On success, endpoints return the requested data directly as JSON:

```json
{
  "field1": "value1",
  "field2": "value2"
}
```

### Error Response

On error, endpoints return:

```json
{
  "status_code": 2001,
  "status_message": "Error description",
  "timestamp": "2024-01-15T10:30:00Z"
}
```

Common status codes:
- `400` - Bad Request (invalid input)
- `401` - Unauthorized (authentication failed)
- `204` - No Content (resource not found or operation failed)
- `500` - Internal Server Error

## Request Timeout

All API requests have a **5-second timeout**. Long-running operations should use the WebSocket interface for progress updates.

## CORS

The API supports CORS with the following headers:
- `Access-Control-Allow-Origin`: Reflects the request origin
- `Access-Control-Allow-Methods`: GET, POST, OPTIONS
- `Access-Control-Allow-Headers`: Content-Type, Authorization

## API Endpoint Groups

### Public Endpoints (No Authentication)

| Endpoint | Description |
|----------|-------------|
| [GET /config/{name}](api-endpoints.md#get-apiv1configname) | Read configuration by name |
| [POST /users/authenticate](api-endpoints.md#post-apiv1usersauthenticate) | Authenticate user |
| [POST /users/register](api-endpoints.md#post-apiv1usersregister) | Register new user |

### Protected Endpoints (Authentication Required)

#### Users
| Endpoint | Description |
|----------|-------------|
| [GET /users/info/{name}](api-endpoints.md#get-apiv1usersinfoname) | Get user information |
| [GET /users/list](api-endpoints.md#get-apiv1userslist) | List all users |
| [POST /users/create](api-endpoints.md#post-apiv1userscreate) | Create new user (admin/operator) |
| [PUT /users/update/{username}](api-endpoints.md#put-apiv1usersupdateusername) | Update user (admin/operator) |
| [DELETE /users/delete/{username}](api-endpoints.md#delete-apiv1usersdeleteusername) | Delete user (admin/operator) |

#### Locations & Charge Points
| Endpoint | Description |
|----------|-------------|
| [GET /locations](api-endpoints.md#get-apiv1locations) | List all locations |
| [GET /chp](api-endpoints.md#get-apiv1chp) | List all charge points |
| [GET /chp/{search}](api-endpoints.md#get-apiv1chpsearch) | Search charge points |
| [GET /point/{id}](api-endpoints.md#get-apiv1pointid) | Get charge point details |
| [POST /point/{id}](api-endpoints.md#post-apiv1pointid) | Update charge point |

#### Transactions
| Endpoint | Description |
|----------|-------------|
| [GET /transactions/active](api-endpoints.md#get-apiv1transactionsactive) | List active transactions |
| [GET /transactions/list](api-endpoints.md#get-apiv1transactionslist) | List all transactions |
| [GET /transactions/list/{period}](api-endpoints.md#get-apiv1transactionslistperiod) | List transactions by period |
| [GET /transactions/recent](api-endpoints.md#get-apiv1transactionsrecent) | Get recent charge points for user |
| [GET /transactions/info/{id}](api-endpoints.md#get-apiv1transactionsinfoid) | Get transaction details |

#### Payments
| Endpoint | Description |
|----------|-------------|
| [GET /payment/methods](api-endpoints.md#get-apiv1paymentmethods) | List payment methods |
| [POST /payment/save](api-endpoints.md#post-apiv1paymentsave) | Save payment method |
| [POST /payment/update](api-endpoints.md#post-apiv1paymentupdate) | Update payment method |
| [POST /payment/delete](api-endpoints.md#post-apiv1paymentdelete) | Delete payment method |
| [POST /payment/order](api-endpoints.md#post-apiv1paymentorder) | Create payment order |

#### Reports
| Endpoint | Description |
|----------|-------------|
| [GET /report/month](api-endpoints.md#get-apiv1reportmonth) | Monthly statistics |
| [GET /report/user](api-endpoints.md#get-apiv1reportuser) | User statistics |
| [GET /report/charger](api-endpoints.md#get-apiv1reportcharger) | Charger statistics |

#### Central System
| Endpoint | Description |
|----------|-------------|
| [POST /csc](api-endpoints.md#post-apiv1csc) | Send command to central system |

#### Utility
| Endpoint | Description |
|----------|-------------|
| [GET /log/{name}](api-endpoints.md#get-apiv1logname) | Read log by name |

---

## WebSocket Interface

The WebSocket endpoint at `/ws` provides real-time updates for:
- Transaction state changes
- Charge point events
- Log events

### Connection

Connect to `/ws` to establish a WebSocket connection. After connection, authenticate by sending a request with your token.

### Request Format

```json
{
  "token": "your-auth-token",
  "charge_point_id": "CP001",
  "connector_id": 1,
  "transaction_id": 12345,
  "command": "CommandName"
}
```

### Available Commands

| Command | Description |
|---------|-------------|
| `StartTransaction` | Initiate a charging transaction |
| `StopTransaction` | Stop a charging transaction |
| `CheckStatus` | Check current user status |
| `ListenTransaction` | Subscribe to transaction meter updates |
| `StopListenTransaction` | Unsubscribe from transaction updates |
| `ListenChargePoints` | Subscribe to charge point events |
| `ListenLog` | Subscribe to log events |
| `PingConnection` | Keep connection alive |

### Response Format

```json
{
  "status": "success|error|waiting|ping|value|event",
  "stage": "start|stop|info|log-event|charge-point-event",
  "info": "Human-readable message",
  "id": 12345,
  "progress": 50,
  "power": 1500,
  "power_rate": 7200,
  "soc": 45,
  "price": 250,
  "minute": 15,
  "connector_id": 1,
  "connector_status": "Charging",
  "data": "Additional data",
  "meter_value": { ... }
}
```

### Subscription Types

After authentication, clients can subscribe to different event types:

1. **Broadcast** - Receive all broadcast messages
2. **LogEvent** - Receive log update events
3. **ChargePointEvent** - Receive charge point status changes (default)

### Transaction Flow Example

1. Connect to `/ws`
2. Authenticate with token
3. Send `StartTransaction` command
4. Receive progress updates (`status: waiting`)
5. Receive success notification (`status: success`, `stage: start`)
6. Send `ListenTransaction` to receive meter values
7. Receive periodic meter value updates
8. Send `StopTransaction` to end charging
9. Receive completion notification

---

## Data Models

For detailed request and response schemas, see [API Endpoints Reference](api-endpoints.md).

### Core Entities

- **User** - User account information
- **Location** - Charging station location
- **ChargePoint** - Individual charging station
- **Connector** - Charging connector on a charge point
- **Transaction** - Charging session record
- **PaymentMethod** - Saved payment card
- **PaymentOrder** - Payment transaction

---

## Rate Limiting

Currently, no rate limiting is implemented. The 5-second request timeout provides basic protection against long-running requests.

## Versioning

The API version is included in the URL path (`/api/v1`). Breaking changes will result in a new version number.
