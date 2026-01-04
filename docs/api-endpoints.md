# API Endpoints Reference

Detailed documentation for all REST API endpoints.

**Related documentation:** [API Structure Overview](api-structure.md)

---

## Table of Contents

- [Public Endpoints](#public-endpoints)
  - [GET /config/{name}](#get-apiv1configname)
  - [POST /users/authenticate](#post-apiv1usersauthenticate)
  - [POST /users/register](#post-apiv1usersregister)
- [Users](#users)
  - [GET /users/info/{name}](#get-apiv1usersinfoname)
  - [GET /users/list](#get-apiv1userslist)
  - [POST /users/create](#post-apiv1userscreate)
  - [PUT /users/update/{username}](#put-apiv1usersupdateusername)
  - [DELETE /users/delete/{username}](#delete-apiv1usersdeleteusername)
- [User Tags](#user-tags)
  - [GET /user-tags/list](#get-apiv1user-tagslist)
  - [GET /user-tags/info/{idTag}](#get-apiv1user-tagsinfoidtag)
  - [POST /user-tags/create](#post-apiv1user-tagscreate)
  - [PUT /user-tags/update/{idTag}](#put-apiv1user-tagsupdateidtag)
  - [DELETE /user-tags/delete/{idTag}](#delete-apiv1user-tagsdeleteidtag)
- [Locations](#locations)
  - [GET /locations](#get-apiv1locations)
  - [GET /chp](#get-apiv1chp)
  - [GET /chp/{search}](#get-apiv1chpsearch)
  - [GET /point/{id}](#get-apiv1pointid)
  - [POST /point/{id}](#post-apiv1pointid)
- [Transactions](#transactions)
  - [GET /transactions/active](#get-apiv1transactionsactive)
  - [GET /transactions/list](#get-apiv1transactionslist)
  - [GET /transactions/list/{period}](#get-apiv1transactionslistperiod)
  - [GET /transactions/recent](#get-apiv1transactionsrecent)
  - [GET /transactions/info/{id}](#get-apiv1transactionsinfoid)
- [Payments](#payments)
  - [GET /payment/methods](#get-apiv1paymentmethods)
  - [POST /payment/save](#post-apiv1paymentsave)
  - [POST /payment/update](#post-apiv1paymentupdate)
  - [POST /payment/delete](#post-apiv1paymentdelete)
  - [POST /payment/order](#post-apiv1paymentorder)
- [Reports](#reports)
  - [GET /report/month](#get-apiv1reportmonth)
  - [GET /report/user](#get-apiv1reportuser)
  - [GET /report/charger](#get-apiv1reportcharger)
  - [GET /report/uptime](#get-apiv1reportuptime)
  - [GET /report/status](#get-apiv1reportstatus)
- [Central System](#central-system)
  - [POST /csc](#post-apiv1csc)
- [Utility](#utility)
  - [GET /log/{name}](#get-apiv1logname)
- [WebSocket](#websocket)
  - [WebSocket Request](#websocket-request)
  - [WebSocket Response](#websocket-response)

---

## Public Endpoints

These endpoints do not require authentication.

### GET /api/v1/config/{name}

Retrieve configuration by name.

**Path Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| name | string | Yes | Configuration name |

**Response:**

Returns configuration data as JSON object. Structure varies based on configuration name.

**Error Response:**

```json
{
  "status_code": 2001,
  "status_message": "Failed to get config: <error>",
  "timestamp": "2024-01-15T10:30:00Z"
}
```

---

### POST /api/v1/users/authenticate

Authenticate user with username/password or token.

**Request Body:**

```json
{
  "username": "user@example.com",
  "password": "password123"
}
```

Or authenticate with token only:

```json
{
  "password": "your-auth-token"
}
```

**Request Fields:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| username | string | No | Username (email). If empty, authenticates by token. |
| password | string | Yes | Password or authentication token |

**Success Response:**

```json
{
  "username": "user@example.com",
  "name": "John Doe",
  "role": "user",
  "access_level": 1,
  "email": "user@example.com",
  "payment_plan": "standard",
  "group": "default",
  "token": "abc123...",
  "user_id": "uid123...",
  "date_registered": "2024-01-01T00:00:00Z",
  "last_seen": "2024-01-15T10:30:00Z"
}
```

**Response Fields:**

| Field | Type | Description |
|-------|------|-------------|
| username | string | User's username |
| name | string | User's display name |
| role | string | User role: `admin`, `operator`, or `user` |
| access_level | integer | Access level (0-10) |
| email | string | Email address |
| payment_plan | string | Active payment plan ID |
| group | string | User group |
| token | string | Authentication token for subsequent requests |
| user_id | string | Unique user identifier |
| date_registered | string | Registration timestamp (ISO 8601) |
| last_seen | string | Last activity timestamp (ISO 8601) |

**Error Response (401):**

```json
{
  "status_code": 2001,
  "status_message": "Not authorized: <error>",
  "timestamp": "2024-01-15T10:30:00Z"
}
```

---

### POST /api/v1/users/register

Register a new user account.

**Request Body:**

```json
{
  "username": "user@example.com",
  "password": "password123",
  "name": "John Doe",
  "email": "user@example.com"
}
```

**Request Fields:**

| Field | Type | Required | Validation | Description |
|-------|------|----------|------------|-------------|
| username | string | No | - | Username |
| password | string | Yes | Required | Password |
| name | string | No | - | Display name |
| email | string | No | - | Email address |

**Success Response:**

Returns the created [User](#user-object) object with generated `token` and `user_id`.

**Error Response (400):**

```json
{
  "status_code": 2001,
  "status_message": "Failed to decode user data: <error>",
  "timestamp": "2024-01-15T10:30:00Z"
}
```

**Error Response (500):**

```json
{
  "status_code": 2001,
  "status_message": "Failed to save user: <error>",
  "timestamp": "2024-01-15T10:30:00Z"
}
```

---

## Users

All endpoints in this section require authentication.

### GET /api/v1/users/info/{name}

Get detailed information about a specific user.

**Path Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| name | string | Yes | Username to look up |

**Success Response:**

```json
{
  "username": "user@example.com",
  "name": "John Doe",
  "role": "user",
  "access_level": 1,
  "email": "user@example.com",
  "date_registered": "2024-01-01T00:00:00Z",
  "last_seen": "2024-01-15T10:30:00Z",
  "payment_plans": [
    {
      "plan_id": "standard",
      "description": "Standard Plan",
      "is_default": true,
      "is_active": true,
      "price_per_kwh": 35,
      "price_per_hour": 0,
      "start_time": "00:00",
      "end_time": "23:59"
    }
  ],
  "user_tags": [
    {
      "username": "user@example.com",
      "user_id": "uid123",
      "id_tag": "TAG001",
      "source": "mobile",
      "is_enabled": true,
      "local": false,
      "note": "",
      "date_registered": "2024-01-01T00:00:00Z",
      "last_seen": "2024-01-15T10:30:00Z"
    }
  ],
  "payment_methods": [
    {
      "description": "My Visa",
      "identifier": "pm_123",
      "card_number": "****1234",
      "card_type": "credit",
      "card_brand": "visa",
      "card_country": "US",
      "expiry_date": "12/25",
      "is_default": true
    }
  ]
}
```

**Response Fields (UserInfo):**

| Field | Type | Description |
|-------|------|-------------|
| username | string | Username |
| name | string | Display name |
| role | string | User role |
| access_level | integer | Access level (0-10) |
| email | string | Email address |
| date_registered | string | Registration date (ISO 8601) |
| last_seen | string | Last activity (ISO 8601) |
| payment_plans | array | Array of [PaymentPlan](#paymentplan-object) objects |
| user_tags | array | Array of [UserTag](#usertag-object) objects |
| payment_methods | array | Array of [PaymentMethod](#paymentmethod-object) objects |

---

### GET /api/v1/users/list

List all users. Results may be filtered based on the requesting user's role and access level.

**Success Response:**

Returns an array of [User](#user-object) objects.

```json
[
  {
    "username": "user1@example.com",
    "name": "User One",
    "role": "user",
    "access_level": 1,
    "email": "user1@example.com",
    "date_registered": "2024-01-01T00:00:00Z",
    "last_seen": "2024-01-15T10:30:00Z"
  }
]
```

---

### POST /api/v1/users/create

Create a new user account (admin/operator only).

**Request Body:**

```json
{
  "username": "newuser@example.com",
  "password": "password123",
  "name": "New User",
  "email": "newuser@example.com",
  "role": "user",
  "access_level": 1,
  "payment_plan": "standard"
}
```

**Request Fields:**

| Field | Type | Required | Validation | Description |
|-------|------|----------|------------|-------------|
| username | string | Yes | Min 3 chars, unique | Username |
| password | string | Yes | Min 6 chars | Password |
| name | string | No | - | Display name |
| email | string | No | Valid email format | Email address |
| role | string | No | `admin`, `operator`, or empty | User role (empty = regular user) |
| access_level | integer | No | 0-10, default: 0 | Access level |
| payment_plan | string | No | - | Payment plan ID |

**Success Response (201 Created):**

```json
{
  "user_id": "abc123...",
  "username": "newuser@example.com",
  "name": "New User",
  "email": "newuser@example.com",
  "role": "user",
  "access_level": 1,
  "payment_plan": "standard",
  "date_registered": "2024-01-15T10:30:00Z"
}
```

**Error Responses:**

| Status | Description |
|--------|-------------|
| 400 | Invalid input data or username already exists |
| 403 | Insufficient permissions (not admin/operator) |

---

### PUT /api/v1/users/update/{username}

Update an existing user's information (admin/operator only).

**Path Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| username | string | Yes | Username of the user to update |

**Request Body:**

All fields are optional. Only provided fields will be updated.

```json
{
  "name": "Updated Name",
  "email": "updated@example.com",
  "password": "newpassword123",
  "role": "operator",
  "access_level": 5,
  "payment_plan": "premium"
}
```

**Request Fields:**

| Field | Type | Required | Validation | Description |
|-------|------|----------|------------|-------------|
| name | string | No | - | Display name |
| email | string | No | Valid email format | Email address |
| password | string | No | Min 6 chars | New password (only if changing) |
| role | string | No | `admin`, `operator`, or empty | User role |
| access_level | integer | No | 0-10 | Access level |
| payment_plan | string | No | - | Payment plan ID |

**Note:** Username cannot be changed.

**Success Response (200 OK):**

```json
{
  "user_id": "abc123...",
  "username": "user@example.com",
  "name": "Updated Name",
  "email": "updated@example.com",
  "role": "operator",
  "access_level": 5,
  "payment_plan": "premium",
  "date_registered": "2024-01-01T00:00:00Z",
  "last_seen": "2024-01-15T10:30:00Z"
}
```

**Error Responses:**

| Status | Description |
|--------|-------------|
| 400 | Invalid input data |
| 403 | Insufficient permissions (not admin/operator) |
| 404 | User not found |

---

### DELETE /api/v1/users/delete/{username}

Delete a user account (admin/operator only).

**Path Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| username | string | Yes | Username of the user to delete |

**Success Response (200 OK):**

```json
{
  "success": true,
  "message": "User deleted successfully"
}
```

**Error Responses:**

| Status | Description |
|--------|-------------|
| 403 | Insufficient permissions (not admin/operator) |
| 404 | User not found |

---

## User Tags

All user tag endpoints require authentication and admin/operator role.

### GET /api/v1/user-tags/list

List all user tags in the system.

**Success Response:**

Returns an array of [UserTag](#usertag-object) objects.

```json
[
  {
    "username": "user@example.com",
    "user_id": "uid123",
    "id_tag": "TAG001",
    "source": "manual",
    "is_enabled": true,
    "local": false,
    "note": "Main RFID card",
    "date_registered": "2024-01-01T00:00:00Z",
    "last_seen": "2024-01-15T10:30:00Z"
  }
]
```

**Error Responses:**

| Status | Description |
|--------|-------------|
| 401 | Not authenticated |
| 403 | Insufficient permissions (not admin/operator) |

---

### GET /api/v1/user-tags/info/{idTag}

Get details for a specific user tag.

**Path Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| idTag | string | Yes | The tag identifier |

**Success Response:**

Returns a [UserTag](#usertag-object) object.

```json
{
  "username": "user@example.com",
  "user_id": "uid123",
  "id_tag": "TAG001",
  "source": "manual",
  "is_enabled": true,
  "local": false,
  "note": "Main RFID card",
  "date_registered": "2024-01-01T00:00:00Z",
  "last_seen": "2024-01-15T10:30:00Z"
}
```

**Error Responses:**

| Status | Description |
|--------|-------------|
| 401 | Not authenticated |
| 403 | Insufficient permissions (not admin/operator) |
| 404 | Tag not found |

---

### POST /api/v1/user-tags/create

Create a new user tag.

**Request Body:**

```json
{
  "username": "user@example.com",
  "user_id": "uid123",
  "id_tag": "TAG002",
  "source": "manual",
  "is_enabled": true,
  "local": false,
  "note": "Secondary card"
}
```

**Request Fields:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| username | string | Yes | Associated username |
| user_id | string | No | User ID (auto-populated from username if not provided) |
| id_tag | string | Yes | Unique tag identifier (e.g., RFID number) |
| source | string | No | Tag source/origin |
| is_enabled | boolean | No | Whether tag is active (default: true) |
| local | boolean | No | Local flag (default: false) |
| note | string | No | Administrative notes |

**Success Response (201 Created):**

Returns the created [UserTag](#usertag-object) object.

**Error Responses:**

| Status | Description |
|--------|-------------|
| 400 | Invalid input data or id_tag already exists |
| 401 | Not authenticated |
| 403 | Insufficient permissions (not admin/operator) |
| 404 | Username not found |

---

### PUT /api/v1/user-tags/update/{idTag}

Update an existing user tag.

**Path Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| idTag | string | Yes | The tag identifier to update |

**Request Body:**

All fields are optional. Only provided fields will be updated.

```json
{
  "username": "newuser@example.com",
  "source": "rfid",
  "is_enabled": false,
  "local": true,
  "note": "Updated note"
}
```

**Request Fields:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| username | string | No | Change associated user |
| source | string | No | Tag source/origin |
| is_enabled | boolean | No | Whether tag is active |
| local | boolean | No | Local flag |
| note | string | No | Administrative notes |

**Note:** The `id_tag` cannot be changed after creation.

**Success Response (200 OK):**

Returns the updated [UserTag](#usertag-object) object.

**Error Responses:**

| Status | Description |
|--------|-------------|
| 400 | Invalid input data |
| 401 | Not authenticated |
| 403 | Insufficient permissions (not admin/operator) |
| 404 | Tag not found or username not found |

---

### DELETE /api/v1/user-tags/delete/{idTag}

Delete a user tag.

**Path Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| idTag | string | Yes | The tag identifier to delete |

**Success Response (200 OK):**

```json
{
  "success": true,
  "message": "Tag deleted successfully"
}
```

**Error Responses:**

| Status | Description |
|--------|-------------|
| 401 | Not authenticated |
| 403 | Insufficient permissions (not admin/operator) |
| 404 | Tag not found |

---

## Locations

### GET /api/v1/locations

List all locations accessible to the authenticated user.

**Success Response:**

Returns an array of [Location](#location-object) objects.

```json
[
  {
    "id": "LOC001",
    "roaming": false,
    "name": "Main Station",
    "address": "123 Main St",
    "city": "New York",
    "postal_code": "10001",
    "country": "USA",
    "coordinates": {
      "latitude": 40.7128,
      "longitude": -74.0060
    },
    "power_limit": 150000,
    "default_power_limit": 50000,
    "charge_points": [...]
  }
]
```

---

### GET /api/v1/chp

List all charge points accessible to the authenticated user.

**Success Response:**

Returns an array of [ChargePoint](#chargepoint-object) objects.

---

### GET /api/v1/chp/{search}

Search charge points by ID or other criteria.

**Path Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| search | string | Yes | Search query (charge point ID, title, etc.) |

**Success Response:**

Returns an array of matching [ChargePoint](#chargepoint-object) objects.

---

### GET /api/v1/point/{id}

Get detailed information about a specific charge point.

**Path Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| id | string | Yes | Charge point ID |

**Success Response:**

```json
{
  "charge_point_id": "CP001",
  "is_enabled": true,
  "title": "Fast Charger 1",
  "description": "50kW DC fast charger",
  "model": "Model X",
  "serial_number": "SN123456",
  "vendor": "ChargerCo",
  "firmware_version": "1.2.3",
  "status": "Available",
  "error_code": "",
  "info": "",
  "event_time": "2024-01-15T10:30:00Z",
  "is_online": true,
  "status_time": "2024-01-15T10:30:00Z",
  "address": "123 Main St",
  "access_type": "public",
  "access_level": 0,
  "location": {
    "latitude": 40.7128,
    "longitude": -74.0060
  },
  "connectors": [
    {
      "connector_id": 1,
      "connector_id_name": "CCS",
      "charge_point_id": "CP001",
      "type": "CCS",
      "status": "Available",
      "status_time": "2024-01-15T10:30:00Z",
      "state": "available",
      "info": "",
      "vendor_id": "",
      "error_code": "",
      "power": 50000,
      "current_transaction_id": 0
    }
  ]
}
```

---

### POST /api/v1/point/{id}

Update a charge point's configuration.

**Path Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| id | string | Yes | Charge point ID |

**Request Body:**

[ChargePoint](#chargepoint-object) object with fields to update.

```json
{
  "charge_point_id": "CP001",
  "is_enabled": true,
  "title": "Updated Title",
  "description": "Updated description"
}
```

**Success Response:**

Returns the updated [ChargePoint](#chargepoint-object) object.

---

## Transactions

### GET /api/v1/transactions/active

List all active (in-progress) charging sessions for the authenticated user.

**Success Response:**

Returns an array of [ChargeState](#chargestate-object) objects representing active charging sessions.

```json
[
  {
    "transaction_id": 12345,
    "connector_id": 1,
    "connector": {
      "connector_id": 1,
      "type": "CCS",
      "status": "Charging",
      "power": 50000
    },
    "charge_point_id": "CP001",
    "charge_point_title": "Fast Charger 1",
    "charge_point_address": "123 Main St",
    "time_started": "2024-01-15T09:00:00Z",
    "meter_start": 0,
    "duration": 1800,
    "consumed": 7500,
    "power_rate": 7200,
    "price": 263,
    "payment_billed": 0,
    "status": "Charging",
    "is_charging": true,
    "can_stop": true,
    "meter_values": [
      {
        "transaction_id": 12345,
        "value": 7500,
        "power_rate": 7200,
        "consumed_energy": 7500,
        "price": 263,
        "time": "2024-01-15T09:30:00Z",
        "minute": 30
      }
    ],
    "id_tag": "TAG001",
    "payment_plan": {
      "plan_id": "standard",
      "price_per_kwh": 35
    },
    "payment_method": {
      "identifier": "pm_123abc",
      "card_brand": "visa",
      "card_number": "****1234"
    },
    "payment_orders": []
  }
]
```

**Response Fields:**

See [ChargeState Object](#chargestate-object) for field descriptions.

---

### GET /api/v1/transactions/list

List transactions for the authenticated user, with optional filtering for admin/operator users.

**Query Parameters (Admin/Operator only):**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| from | string | No | Start date filter (inclusive), format: `YYYY-MM-DD` |
| to | string | No | End date filter (inclusive), format: `YYYY-MM-DD` or `YYYY-MM-DDTHH:mm:ss` |
| username | string | No | Filter by user's username (via UserTag.username) |
| id_tag | string | No | Filter by RFID tag ID |
| charge_point_id | string | No | Filter by charge point identifier |

**Behavior:**

- **Regular users**: Returns the user's own transactions (no filters applied)
- **Admin/Operator with filters**: Returns filtered transactions across all users
- **Admin/Operator without filters**: Returns the user's own transactions (legacy behavior)

**Example Requests:**

```
GET /api/v1/transactions/list?from=2024-01-01&to=2024-01-31T23:59:59
GET /api/v1/transactions/list?username=john.doe
GET /api/v1/transactions/list?charge_point_id=CP001
GET /api/v1/transactions/list?from=2024-01-01&to=2024-01-31&charge_point_id=CP001
```

**Success Response:**

Returns an array of [Transaction](#transaction-object) objects, sorted by `time_start` descending (newest first).

---

### GET /api/v1/transactions/list/{period}

List transactions filtered by time period.

**Path Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| period | string | Yes | Time period filter (e.g., "day", "week", "month") |

**Success Response:**

Returns an array of [Transaction](#transaction-object) objects within the specified period.

---

### GET /api/v1/transactions/recent

Get recent charge points used by the authenticated user.

**Success Response:**

Returns charge points from the user's recent transactions.

---

### GET /api/v1/transactions/info/{id}

Get detailed state information about a specific transaction.

**Path Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| id | integer | Yes | Transaction ID |

**Success Response:**

Returns a [ChargeState](#chargestate-object) object with transaction details and current state.

```json
{
  "transaction_id": 12345,
  "connector_id": 1,
  "connector": {
    "connector_id": 1,
    "type": "CCS",
    "status": "Finishing",
    "power": 50000
  },
  "charge_point_id": "CP001",
  "charge_point_title": "Fast Charger 1",
  "charge_point_address": "123 Main St",
  "time_started": "2024-01-15T09:00:00Z",
  "meter_start": 0,
  "duration": 5400,
  "consumed": 15000,
  "power_rate": 0,
  "price": 525,
  "payment_billed": 525,
  "status": "Finishing",
  "is_charging": false,
  "can_stop": false,
  "meter_values": [
    {
      "transaction_id": 12345,
      "value": 15000,
      "power_rate": 7200,
      "battery_level": 80,
      "consumed_energy": 15000,
      "price": 525,
      "time": "2024-01-15T10:30:00Z",
      "minute": 90
    }
  ],
  "id_tag": "TAG001",
  "payment_plan": {
    "plan_id": "standard",
    "description": "Standard Plan",
    "price_per_kwh": 35
  },
  "payment_method": {
    "identifier": "pm_123abc",
    "card_brand": "visa",
    "card_number": "****1234"
  },
  "payment_orders": [
    {
      "order": 1001,
      "amount": 525,
      "is_completed": true
    }
  ]
}
```

**Response Fields:**

See [ChargeState Object](#chargestate-object) for field descriptions.

---

## Payments

### GET /api/v1/payment/methods

List saved payment methods for the authenticated user.

**Success Response:**

Returns an array of [PaymentMethod](#paymentmethod-object) objects.

```json
[
  {
    "description": "My Visa Card",
    "identifier": "pm_123abc",
    "card_number": "****1234",
    "card_type": "credit",
    "card_brand": "visa",
    "card_country": "US",
    "expiry_date": "12/25",
    "is_default": true,
    "user_id": "uid123",
    "user_name": "user@example.com"
  }
]
```

---

### POST /api/v1/payment/save

Save a new payment method.

**Request Body:**

```json
{
  "description": "My New Card",
  "identifier": "pm_new123"
}
```

**Request Fields:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| description | string | No | Friendly name for the payment method |
| identifier | string | Yes | Payment method identifier from payment provider |

**Success Response:**

Returns the saved [PaymentMethod](#paymentmethod-object) object.

---

### POST /api/v1/payment/update

Update an existing payment method.

**Request Body:**

```json
{
  "identifier": "pm_123abc",
  "description": "Updated Card Name",
  "is_default": true
}
```

**Success Response:**

Returns the updated [PaymentMethod](#paymentmethod-object) object.

---

### POST /api/v1/payment/delete

Delete a payment method.

**Request Body:**

```json
{
  "identifier": "pm_123abc"
}
```

**Success Response:**

Returns the deleted [PaymentMethod](#paymentmethod-object) object.

---

### POST /api/v1/payment/order

Create a payment order for a transaction.

**Request Body:**

```json
{
  "transaction_id": 12345,
  "amount": 525,
  "currency": "USD",
  "description": "Charging session payment",
  "identifier": "pm_123abc"
}
```

**Request Fields:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| transaction_id | integer | No | Associated transaction ID |
| amount | integer | No | Payment amount in cents |
| currency | string | No | Currency code (e.g., "USD") |
| description | string | No | Payment description |
| identifier | string | No | Payment method identifier |

**Success Response:**

```json
{
  "transaction_id": 12345,
  "order": 1001,
  "user_id": "uid123",
  "user_name": "user@example.com",
  "amount": 525,
  "currency": "USD",
  "description": "Charging session payment",
  "identifier": "pm_123abc",
  "is_completed": false,
  "result": "",
  "date": "2024-01-15",
  "time_opened": "2024-01-15T10:30:00Z",
  "time_closed": "0001-01-01T00:00:00Z"
}
```

---

## Reports

All report endpoints accept the following query parameters:

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| from | string | Yes | Start date (YYYY-MM-DD) |
| to | string | Yes | End date (YYYY-MM-DD) |
| group | string | No | User group filter |

### GET /api/v1/report/month

Get monthly aggregated statistics.

**Query Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| from | string | Yes | Start date |
| to | string | Yes | End date |
| group | string | No | User group filter |

**Success Response:**

Returns an array of monthly statistics objects.

---

### GET /api/v1/report/user

Get user-based statistics.

**Query Parameters:**

Same as [GET /report/month](#get-apiv1reportmonth).

**Success Response:**

Returns an array of per-user statistics objects.

---

### GET /api/v1/report/charger

Get charger-based statistics.

**Query Parameters:**

Same as [GET /report/month](#get-apiv1reportmonth).

**Success Response:**

Returns an array of per-charger statistics objects.

---

### GET /api/v1/report/uptime

Get station uptime/downtime statistics over a period.

**Query Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| from | string | Yes | Start date (RFC3339 format, e.g., `2024-01-01T00:00:00Z`) |
| to | string | Yes | End date (RFC3339 format, e.g., `2024-01-31T23:59:59Z`) |
| charge_point_id | string | No | Filter by specific charge point ID |

**Note:** `to` must be after `from`.

**Success Response:**

Returns an array of [StationUptime](#stationuptime-object) objects.

```json
[
  {
    "charge_point_id": "CP001",
    "online_seconds": 2592000,
    "offline_seconds": 86400,
    "online_minutes": 43200.0,
    "offline_minutes": 1440.0,
    "uptime_percent": 96.77,
    "final_state": "ONLINE"
  },
  {
    "charge_point_id": "CP002",
    "online_seconds": 2678400,
    "offline_seconds": 0,
    "online_minutes": 44640.0,
    "offline_minutes": 0.0,
    "uptime_percent": 100.0,
    "final_state": "ONLINE"
  }
]
```

**Error Responses:**

| Status | Description |
|--------|-------------|
| 400 | Invalid parameters or `to` before `from` |
| 401 | Not authenticated |
| 403 | Insufficient permissions (requires reports access) |

---

### GET /api/v1/report/status

Get current connection status for stations.

**Query Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| charge_point_id | string | No | Filter by specific charge point ID |

**Success Response:**

Returns an array of [StationStatus](#stationstatus-object) objects.

```json
[
  {
    "charge_point_id": "CP001",
    "state": "ONLINE",
    "since": "2024-01-30T14:30:00Z",
    "duration_seconds": 86400,
    "duration_minutes": 1440.0,
    "last_event_text": "registered"
  },
  {
    "charge_point_id": "CP002",
    "state": "OFFLINE",
    "since": "2024-01-31T08:15:00Z",
    "duration_seconds": 3600,
    "duration_minutes": 60.0,
    "last_event_text": "unregistered"
  }
]
```

**Error Responses:**

| Status | Description |
|--------|-------------|
| 400 | Invalid parameters |
| 401 | Not authenticated |
| 403 | Insufficient permissions (requires reports access) |

---

## Central System

### POST /api/v1/csc

Send a command to the central system (OCPP backend).

**Request Body:**

```json
{
  "charge_point_id": "CP001",
  "connector_id": 1,
  "feature_name": "RemoteStartTransaction",
  "payload": "TAG001"
}
```

**Request Fields:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| charge_point_id | string | No | Target charge point ID |
| connector_id | integer | No | Target connector ID |
| feature_name | string | Yes | OCPP feature name |
| payload | string | No | Command payload/parameters |

**Common Feature Names:**

| Feature | Description | Payload |
|---------|-------------|---------|
| RemoteStartTransaction | Start charging | ID tag string |
| RemoteStopTransaction | Stop charging | Transaction ID |
| Reset | Reset charge point | "Soft" or "Hard" |
| UnlockConnector | Unlock connector | - |
| GetConfiguration | Get configuration | - |
| ChangeConfiguration | Change configuration | Key=Value |

**Success Response:**

Returns the central system response (varies by command).

---

## Utility

### GET /api/v1/log/{name}

Read log entries by name.

**Path Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| name | string | Yes | Log name/category |

**Success Response:**

Returns log entries (structure varies).

---

## WebSocket

Connect to `/ws` for real-time updates. See [API Structure](api-structure.md#websocket-interface) for connection details.

### WebSocket Request

```json
{
  "token": "your-auth-token",
  "charge_point_id": "CP001",
  "connector_id": 1,
  "transaction_id": 12345,
  "command": "StartTransaction"
}
```

**Fields:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| token | string | Yes | Authentication token |
| charge_point_id | string | No | Target charge point |
| connector_id | integer | No | Target connector |
| transaction_id | integer | No | Transaction ID (for stop/listen) |
| command | string | Yes | Command name |

**Commands:**

| Command | Description |
|---------|-------------|
| StartTransaction | Start charging session |
| StopTransaction | Stop charging session |
| CheckStatus | Check user's current status |
| ListenTransaction | Subscribe to meter value updates |
| StopListenTransaction | Unsubscribe from updates |
| ListenChargePoints | Subscribe to charge point events |
| ListenLog | Subscribe to log events |
| PingConnection | Keep-alive ping |

---

### WebSocket Response

```json
{
  "status": "success",
  "stage": "start",
  "info": "Transaction started: 12345",
  "id": 12345,
  "progress": 100,
  "power": 15000,
  "power_rate": 7200,
  "soc": 45,
  "price": 175,
  "minute": 30,
  "connector_id": 1,
  "connector_status": "Charging",
  "data": "",
  "meter_value": {
    "transaction_id": 12345,
    "value": 15000,
    "power_rate": 7200,
    "battery_level": 45,
    "consumed_energy": 15000,
    "price": 175,
    "time": "2024-01-15T09:30:00Z",
    "timestamp": 1705312200,
    "minute": 30,
    "unit": "Wh",
    "measurand": "Energy.Active.Import.Register",
    "connector_id": 1,
    "connector_status": "Charging"
  }
}
```

**Status Values:**

| Status | Description |
|--------|-------------|
| success | Operation completed successfully |
| error | Operation failed |
| waiting | Operation in progress |
| ping | Keep-alive response |
| value | Meter value update |
| event | Event notification |

**Stage Values:**

| Stage | Description |
|-------|-------------|
| start | Transaction start phase |
| stop | Transaction stop phase |
| info | Information message |
| log-event | Log event subscription |
| charge-point-event | Charge point event subscription |

---

## Data Objects Reference

### User Object

```json
{
  "username": "string",
  "password": "string",
  "name": "string",
  "role": "string",
  "access_level": 0,
  "email": "string",
  "payment_plan": "string",
  "group": "string",
  "token": "string",
  "user_id": "string",
  "date_registered": "2024-01-01T00:00:00Z",
  "last_seen": "2024-01-15T10:30:00Z"
}
```

### Location Object

```json
{
  "id": "string (max 39 chars)",
  "roaming": false,
  "name": "string (max 255 chars)",
  "address": "string (max 45 chars)",
  "city": "string (max 45 chars)",
  "postal_code": "string (max 10 chars)",
  "country": "string (ISO 3166-1 alpha-3)",
  "coordinates": { "latitude": 0.0, "longitude": 0.0 },
  "power_limit": 0,
  "default_power_limit": 0,
  "charge_points": []
}
```

### ChargePoint Object

```json
{
  "charge_point_id": "string",
  "is_enabled": true,
  "title": "string",
  "description": "string",
  "model": "string",
  "serial_number": "string",
  "vendor": "string",
  "firmware_version": "string",
  "status": "string",
  "error_code": "string",
  "info": "string",
  "event_time": "2024-01-01T00:00:00Z",
  "is_online": true,
  "status_time": "2024-01-01T00:00:00Z",
  "address": "string",
  "access_type": "string",
  "access_level": 0,
  "location": { "latitude": 0.0, "longitude": 0.0 },
  "connectors": []
}
```

### Connector Object

```json
{
  "connector_id": 1,
  "connector_id_name": "string",
  "charge_point_id": "string",
  "type": "string",
  "status": "string",
  "status_time": "2024-01-01T00:00:00Z",
  "state": "string",
  "info": "string",
  "vendor_id": "string",
  "error_code": "string",
  "power": 0,
  "current_transaction_id": 0
}
```

### ChargeState Object

Represents an active or recent charging session with real-time state information.

```json
{
  "transaction_id": 0,
  "connector_id": 0,
  "connector": {},
  "charge_point_id": "string",
  "charge_point_title": "string",
  "charge_point_address": "string",
  "time_started": "2024-01-01T00:00:00Z",
  "meter_start": 0,
  "duration": 0,
  "consumed": 0,
  "power_rate": 0,
  "price": 0,
  "payment_billed": 0,
  "status": "string",
  "is_charging": true,
  "can_stop": true,
  "meter_values": [],
  "id_tag": "string",
  "payment_plan": {},
  "payment_method": {},
  "payment_orders": []
}
```

| Field | Type | Description |
|-------|------|-------------|
| transaction_id | integer | Unique transaction identifier |
| connector_id | integer | Connector being used |
| connector | object | [Connector](#connector-object) details |
| charge_point_id | string | Charge point identifier |
| charge_point_title | string | Charge point display name |
| charge_point_address | string | Charge point address |
| time_started | string | Session start time (ISO 8601) |
| meter_start | integer | Starting meter value (Wh) |
| duration | integer | Session duration in seconds |
| consumed | integer | Energy consumed (Wh) |
| power_rate | integer | Current power rate (W) |
| price | integer | Current price in cents |
| payment_billed | integer | Amount already billed in cents |
| status | string | Connector status (e.g., Charging, Finishing) |
| is_charging | boolean | Whether actively charging |
| can_stop | boolean | Whether current user can stop the session |
| meter_values | array | Array of [TransactionMeter](#transactionmeter-object) readings |
| id_tag | string | RFID tag used to start the session |
| payment_plan | object | [PaymentPlan](#paymentplan-object) applied |
| payment_method | object | [PaymentMethod](#paymentmethod-object) used |
| payment_orders | array | Array of [PaymentOrder](#paymentorder-object) records |

### Transaction Object

```json
{
  "transaction_id": 0,
  "session_id": "string",
  "is_finished": false,
  "connector_id": 0,
  "charge_point_id": "string",
  "id_tag": "string",
  "reservation_id": 0,
  "meter_start": 0,
  "meter_stop": 0,
  "time_start": "2024-01-01T00:00:00Z",
  "time_stop": "2024-01-01T00:00:00Z",
  "reason": "string",
  "id_tag_note": "string",
  "username": "string",
  "payment_amount": 0,
  "payment_billed": 0,
  "payment_order": 0,
  "payment_plan": {},
  "tariff": {},
  "meter_values": [],
  "payment_method": {},
  "payment_orders": [],
  "user_tag": {},
  "protocol_version": "string",
  "evse_id": 0,
  "metadata": {}
}
```

| Field | Type | Description |
|-------|------|-------------|
| session_id | string | Unique session identifier |
| reason | string | Transaction stop reason |
| id_tag_note | string | Note about the ID tag |
| username | string | Associated username |
| tariff | object | Pricing tariff details |
| payment_method | object | Payment method used |
| payment_orders | array | Multiple payment orders |
| protocol_version | string | OCPP version (ocpp1.6, ocpp2.0.1, ocpp2.1) |
| evse_id | integer | EVSE ID (OCPP 2.0+) |
| metadata | object | Flexible metadata storage |

### Tariff Object

```json
{
  "tariff_id": "string",
  "description": "string"
}
```

### PaymentMethod Object

```json
{
  "description": "string",
  "identifier": "string (required)",
  "card_number": "string",
  "card_type": "string",
  "card_brand": "string",
  "card_country": "string",
  "expiry_date": "string",
  "is_default": false,
  "user_id": "string",
  "user_name": "string",
  "fail_count": 0,
  "merchant_cof_txnid": "string"
}
```

### PaymentOrder Object

```json
{
  "transaction_id": 0,
  "order": 0,
  "user_id": "string",
  "user_name": "string",
  "amount": 0,
  "currency": "string",
  "description": "string",
  "identifier": "string",
  "is_completed": false,
  "result": "string",
  "date": "string",
  "time_opened": "2024-01-01T00:00:00Z",
  "time_closed": "2024-01-01T00:00:00Z",
  "refund_amount": 0,
  "refund_time": "2024-01-01T00:00:00Z"
}
```

### PaymentPlan Object

```json
{
  "plan_id": "string",
  "description": "string",
  "is_default": false,
  "is_active": false,
  "price_per_kwh": 0,
  "price_per_hour": 0,
  "start_time": "string",
  "end_time": "string"
}
```

### UserTag Object

```json
{
  "username": "string",
  "user_id": "string",
  "id_tag": "string",
  "source": "string",
  "is_enabled": true,
  "local": false,
  "note": "string",
  "date_registered": "2024-01-01T00:00:00Z",
  "last_seen": "2024-01-01T00:00:00Z"
}
```

### TransactionMeter Object

```json
{
  "transaction_id": 0,
  "value": 0,
  "power_rate": 0,
  "battery_level": 0,
  "consumed_energy": 0,
  "price": 0,
  "time": "2024-01-01T00:00:00Z",
  "timestamp": 0,
  "minute": 0,
  "unit": "string",
  "measurand": "string",
  "connector_id": 0,
  "connector_status": "string"
}
```

### GeoLocation Object

```json
{
  "latitude": 0.0,
  "longitude": 0.0
}
```

### StationUptime Object

Represents uptime/downtime statistics for a station over a period.

```json
{
  "charge_point_id": "string",
  "online_seconds": 0,
  "offline_seconds": 0,
  "online_minutes": 0.0,
  "offline_minutes": 0.0,
  "uptime_percent": 0.0,
  "final_state": "ONLINE"
}
```

| Field | Type | Description |
|-------|------|-------------|
| charge_point_id | string | Station identifier |
| online_seconds | integer | Total online time in seconds |
| offline_seconds | integer | Total offline time in seconds |
| online_minutes | number | Total online time in minutes |
| offline_minutes | number | Total offline time in minutes |
| uptime_percent | number | Uptime percentage (0-100) |
| final_state | string | Station state at end of period: `ONLINE`, `OFFLINE`, or `UNKNOWN` |

### StationStatus Object

Represents the current connection status for a station.

```json
{
  "charge_point_id": "string",
  "state": "ONLINE",
  "since": "2024-01-01T00:00:00Z",
  "duration_seconds": 0,
  "duration_minutes": 0.0,
  "last_event_text": "string"
}
```

| Field | Type | Description |
|-------|------|-------------|
| charge_point_id | string | Station identifier |
| state | string | Current connection state: `ONLINE`, `OFFLINE`, or `UNKNOWN` |
| since | string | When current state began (ISO 8601) |
| duration_seconds | integer | Time in current state in seconds |
| duration_minutes | number | Time in current state in minutes |
| last_event_text | string | Text of last status event (e.g., "registered", "unregistered") |
