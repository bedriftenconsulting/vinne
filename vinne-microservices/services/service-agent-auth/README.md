# Agent Auth Service

## Purpose
This service handles **AUTHENTICATION ONLY** for both **Agents AND Retailers**. Despite the name "agent-auth", it manages authentication for all business users in the RANDCO platform.

## Why One Service for Both?
- Agents and Retailers have identical authentication needs
- Shared JWT token management
- Unified session handling
- Reduced code duplication
- Single point for authentication logic

## Responsibilities
✅ **What this service DOES:**
- **Agent Authentication**
  - Login via email/password or agent_code/password
  - JWT token generation
  - Session management
  
- **Retailer Authentication**
  - Login via email/password, phone/password, or retailer_code/password
  - POS device authentication (retailer_code + PIN + device IMEI)
  - JWT token generation with device tracking
  
- **Common Features**
  - Password reset for both user types
  - Account locking after failed attempts
  - Session revocation and logout
  - Device tracking (especially for POS terminals)

❌ **What this service DOES NOT do:**
- User registration (handled by agent-management-service)
- Profile management (handled by agent-management-service)
- Business relationships (Agent-Retailer links in agent-management-service)
- Commission calculations (handled by commission-service)
- Wallet operations (handled by wallet-service)
- POS/Terminal assignment (handled by terminal-service)

## Authentication Methods

### Agents
```
Login Options:
- Email + Password
- Agent Code (AGT-2025-000001) + Password
```

### Retailers  
```
Login Options:
- Email + Password (if email registered)
- Phone + Password (if phone registered)
- Retailer Code (RTL-2025-0000001) + Password
- POS Login: Retailer Code + PIN + Device IMEI
```

## Database Schema

### Tables
- `agents` - Agent authentication data only
- `retailers` - Retailer authentication data only
- `sessions` - Unified session management for both
- `password_reset_tokens` - Password reset for both

### Key Fields
```sql
agents:
  - id (UUID)
  - agent_code (AGT-YYYY-XXXXXX)
  - email
  - password_hash
  - is_active
  - locked_until

retailers:
  - id (UUID)
  - retailer_code (RTL-YYYY-XXXXXXX)
  - email (optional)
  - phone (optional)
  - password_hash
  - is_active
  - locked_until

sessions:
  - user_id
  - user_type (AGENT or RETAILER)
  - refresh_token
  - device_id (for POS tracking)
  - expires_at
```

## API Endpoints (gRPC)

```protobuf
service AgentAuthService {
  // Agent Authentication
  rpc AgentLogin(AgentLoginRequest) returns (LoginResponse);
  rpc AgentLogout(LogoutRequest) returns (LogoutResponse);
  
  // Retailer Authentication (yes, in agent-auth service)
  rpc RetailerLogin(RetailerLoginRequest) returns (LoginResponse);
  rpc RetailerPOSLogin(POSLoginRequest) returns (LoginResponse);
  rpc RetailerLogout(LogoutRequest) returns (LogoutResponse);
  
  // Common Operations
  rpc RefreshToken(RefreshTokenRequest) returns (RefreshTokenResponse);
  rpc ValidateToken(ValidateTokenRequest) returns (ValidateTokenResponse);
  rpc ChangePassword(ChangePasswordRequest) returns (ChangePasswordResponse);
  
  // Password Reset
  rpc RequestPasswordReset(PasswordResetRequest) returns (PasswordResetResponse);
  rpc ResetPassword(ResetPasswordRequest) returns (ResetPasswordResponse);
  
  // Session Management
  rpc GetActiveSessions(GetActiveSessionsRequest) returns (GetActiveSessionsResponse);
  rpc RevokeSession(RevokeSessionRequest) returns (RevokeSessionResponse);
  rpc RevokeAllSessions(RevokeAllSessionsRequest) returns (RevokeAllSessionsResponse);
}
```

## JWT Token Structure

### Access Token Claims
```json
{
  "sub": "user_id",
  "user_type": "AGENT|RETAILER",
  "user_code": "AGT-2025-000001|RTL-2025-0000001",
  "device_id": "POS-IMEI-123456", // Optional
  "exp": 1234567890,
  "iat": 1234567890
}
```

### Refresh Token
- Stored in database `sessions` table
- Longer expiry (7 days for web, 24 hours for POS)
- Can be revoked immediately

## Integration Points

### With Agent Management Service
```
1. Agent/Retailer Creation:
   agent-management → agent-auth: CreateAuthCredentials
   
2. Profile Updates:
   agent-management → agent-auth: UpdateEmail/Phone
   
3. Account Status:
   agent-management → agent-auth: ActivateAccount/DeactivateAccount
```

### With Terminal Service
```
1. POS Authentication:
   terminal → agent-auth: ValidatePOSLogin
   Returns: JWT with device_id claim
   
2. Device Validation:
   terminal → agent-auth: CheckDeviceSession
```

### With API Gateway
```
1. Token Validation:
   gateway → agent-auth: ValidateToken
   Returns: User details and permissions
   
2. Token Refresh:
   gateway → agent-auth: RefreshToken
```

## Security Features

### Account Protection
- Max 5 failed login attempts
- 30-minute account lock after threshold
- Password complexity requirements
- Secure password hashing (bcrypt)

### Session Security
- Short-lived access tokens (15 minutes)
- Refresh token rotation
- Device fingerprinting for POS
- IP tracking for web sessions
- Concurrent session limits

### POS-Specific Security
- Device IMEI validation
- PIN + Password dual authentication
- 24-hour session timeout
- Offline token support (future)

## Environment Variables
```env
# Database
DB_HOST=localhost
DB_PORT=5432
DB_NAME=agent_auth_db
DB_USER=agent_auth_user
DB_PASSWORD=

# Redis
REDIS_HOST=localhost
REDIS_PORT=6379

# JWT
JWT_SECRET=your-secret-key
JWT_ACCESS_TOKEN_EXPIRY=15m
JWT_REFRESH_TOKEN_EXPIRY=7d
JWT_POS_TOKEN_EXPIRY=24h

# Security
MAX_LOGIN_ATTEMPTS=5
ACCOUNT_LOCK_DURATION=30m
PASSWORD_MIN_LENGTH=8

# gRPC
GRPC_PORT=50052
```

## Development

### Run Migrations
```bash
cd services/service-agent-auth
goose -dir migrations postgres "postgresql://user:pass@localhost/agent_auth_db" up
```

### Start Service
```bash
go run cmd/server/main.go
```

### Test Authentication
```bash
# Test agent login
grpcurl -plaintext -d '{
  "email": "agent@example.com",
  "password": "password123"
}' localhost:50052 AgentAuthService/AgentLogin

# Test retailer login
grpcurl -plaintext -d '{
  "retailer_code": "RTL-2025-0000001",
  "password": "password123"
}' localhost:50052 AgentAuthService/RetailerLogin
```

## Important Notes

⚠️ **Service Naming**: Although called "agent-auth", this service handles BOTH agents and retailers.

⚠️ **No Business Logic**: This service knows nothing about:
- Agent-Retailer relationships
- Commission rates
- Wallet balances
- Territory assignments
- POS device ownership (only tracks device IDs in sessions)

🔐 **Single Responsibility**: Authentication and session management ONLY.

## Related Services
- **agent-management-service**: Manages agent/retailer profiles and relationships
- **terminal-service**: Manages POS devices and assignments
- **wallet-service**: Handles financial operations
- **commission-service**: Calculates commissions