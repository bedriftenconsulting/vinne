# Agent Management Service

## Purpose
This service handles **AGENT AND RETAILER MANAGEMENT** for the RANDCO platform. It manages the complete lifecycle of agents and retailers, their relationships, and business operations.

## Responsibilities
✅ **What this service DOES:**
- **Agent Management**
  - Agent registration and KYC verification
  - Agent profile management and updates
  - Agent performance tracking and analytics
  - Agent wallet oversight (coordinates with wallet service)
  
- **Retailer Management**
  - Retailer onboarding (by RANDCO or by Agent)
  - Retailer profile management
  - Hierarchical relationship management (Agent → Retailers)
  - Independent retailer management (direct RANDCO)
  - Retailer performance tracking
  
- **POS Device Management**
  - Device assignment to retailers
  - Device status monitoring and management
  - Device-retailer relationship tracking
  - Device provisioning workflows
  
- **Business Relationships**
  - Agent-Retailer hierarchical relationships
  - Commission tier assignments
  - Performance analytics and reporting

❌ **What this service DOES NOT do:**
- Authentication (handled by agent-auth-service)
- Commission calculations (handled by commission-service)
- Financial transactions (handled by wallet-service and payment-service)
- Actual wallet operations (coordinates with wallet-service)

## Business Model

### Entity Hierarchy
- **RANDCO** → Top-level operator
- **Agents** → Registered sales partners managing multiple retailers
- **Retailers** → Individual shops/points of sale
- **POS Devices** → Terminal devices tied to specific Retailers

### ID Generation Standards
- **Agent ID**: `AGT-YYYY-XXXXXX` (e.g., AGT-2025-000001)
- **Retailer ID**: `RTL-YYYY-XXXXXXX` (e.g., RTL-2025-0000001)
- **POS Device ID**: `POS-YYYY-XXXXXX` (e.g., POS-2025-000001)

### Key Business Rules
1. Agents can have multiple Retailers under management
2. Retailers can be:
   - Agent-managed: Onboarded and managed by an Agent
   - Independent: Directly managed by RANDCO without an Agent
3. POS devices are assigned to Retailers, not Agents
4. Both RANDCO and Agents can onboard new Retailers
5. Agents can dispatch POS devices to their Retailers

## Database Schema

### Core Tables
- `agents` - Agent profile and business information
- `retailers` - Retailer profile and business information
- `agent_retailers` - Agent-Retailer relationship mapping
- `pos_devices` - POS device information and assignments
- `commission_tiers` - Commission tier structures
- `agent_kyc` - Agent KYC documentation and status
- `retailer_kyc` - Retailer KYC documentation and status

## API Endpoints (gRPC)

```protobuf
service AgentManagementService {
  // Agent Management
  rpc CreateAgent(CreateAgentRequest) returns (CreateAgentResponse);
  rpc GetAgent(GetAgentRequest) returns (GetAgentResponse);
  rpc UpdateAgent(UpdateAgentRequest) returns (UpdateAgentResponse);
  rpc ListAgents(ListAgentsRequest) returns (ListAgentsResponse);
  rpc DeleteAgent(DeleteAgentRequest) returns (DeleteAgentResponse);
  
  // Retailer Management
  rpc CreateRetailer(CreateRetailerRequest) returns (CreateRetailerResponse);
  rpc GetRetailer(GetRetailerRequest) returns (GetRetailerResponse);
  rpc UpdateRetailer(UpdateRetailerRequest) returns (UpdateRetailerResponse);
  rpc ListRetailers(ListRetailersRequest) returns (ListRetailersResponse);
  rpc DeleteRetailer(DeleteRetailerRequest) returns (DeleteRetailerResponse);
  
  // Agent-Retailer Relationships
  rpc AssignRetailerToAgent(AssignRetailerToAgentRequest) returns (AssignRetailerToAgentResponse);
  rpc UnassignRetailerFromAgent(UnassignRetailerFromAgentRequest) returns (UnassignRetailerFromAgentResponse);
  rpc GetAgentRetailers(GetAgentRetailersRequest) returns (GetAgentRetailersResponse);
  
  // POS Device Management
  rpc CreatePOSDevice(CreatePOSDeviceRequest) returns (CreatePOSDeviceResponse);
  rpc AssignPOSDevice(AssignPOSDeviceRequest) returns (AssignPOSDeviceResponse);
  rpc UnassignPOSDevice(UnassignPOSDeviceRequest) returns (UnassignPOSDeviceResponse);
  rpc GetPOSDevice(GetPOSDeviceRequest) returns (GetPOSDeviceResponse);
  rpc ListPOSDevices(ListPOSDevicesRequest) returns (ListPOSDevicesResponse);
  
  
  // KYC Management
  rpc SubmitAgentKYC(SubmitAgentKYCRequest) returns (SubmitAgentKYCResponse);
  rpc SubmitRetailerKYC(SubmitRetailerKYCRequest) returns (SubmitRetailerKYCResponse);
  rpc UpdateKYCStatus(UpdateKYCStatusRequest) returns (UpdateKYCStatusResponse);
  
  // Performance Analytics
  rpc GetAgentPerformance(GetAgentPerformanceRequest) returns (GetAgentPerformanceResponse);
  rpc GetRetailerPerformance(GetRetailerPerformanceRequest) returns (GetRetailerPerformanceResponse);
}
```

## Integration Points

### With Agent Auth Service
```
1. Agent Creation:
   agent-management → agent-auth: CreateAuthCredentials
   
2. Profile Updates:
   agent-management → agent-auth: UpdateEmail/Phone
   
3. Account Status:
   agent-management → agent-auth: ActivateAccount/DeactivateAccount
```

### With Wallet Service
```
1. Wallet Creation:
   agent-management → wallet: CreateAgentWallet
   agent-management → wallet: CreateRetailerWallet
   
2. Wallet Operations:
   Coordinates with wallet service for balance inquiries
   Does not perform actual financial transactions
```

### With Commission Service
```
1. Commission Tier Assignment:
   agent-management → commission: AssignCommissionTier
   
2. Performance Data:
   commission → agent-management: PerformanceMetrics
```

## Environment Variables
```env
# Database
DB_HOST=localhost
DB_PORT=5435
DB_NAME=agent_management_db
DB_USER=agent_mgmt_user
DB_PASSWORD=

# Redis
REDIS_HOST=localhost
REDIS_PORT=6382

# gRPC
GRPC_PORT=50058

# External Services
AGENT_AUTH_SERVICE_URL=localhost:50052
WALLET_SERVICE_URL=localhost:50054
COMMISSION_SERVICE_URL=localhost:50055
```

## Development

### Run Migrations
```bash
cd services/service-agent-management
goose -dir migrations postgres "postgresql://user:pass@localhost/agent_management_db" up
```

### Start Service
```bash
go run cmd/server/main.go
```

### Test Management Operations
```bash
# Test agent creation
grpcurl -plaintext -d '{
  "business_name": "Test Agent Ltd",
  "contact_email": "agent@test.com",
  "contact_phone": "+233241234567",
  "location": "Accra, Ghana"
}' localhost:50053 AgentManagementService/CreateAgent

# Test retailer creation
grpcurl -plaintext -d '{
  "business_name": "Test Shop",
  "owner_name": "John Doe",
  "contact_phone": "+233241234568",
  "location": "Kumasi, Ghana",
  "agent_id": "AGT-2025-000001"
}' localhost:50053 AgentManagementService/CreateRetailer
```

## Related Services
- **agent-auth-service**: Handles agent/retailer authentication
- **wallet-service**: Manages financial wallets
- **commission-service**: Calculates commissions and manages tiers
- **payment-service**: Handles financial transactions