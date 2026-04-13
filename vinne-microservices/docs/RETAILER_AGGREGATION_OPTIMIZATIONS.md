# Retailer Aggregation Optimization Recommendations

## Current Implementation Analysis

The `ListAgentRetailers` endpoint currently:
- Makes 1 call to get retailers list
- Makes 3 gRPC calls **per retailer**:
  1. GetRetailerWalletBalance
  2. ListPOSDevices  
  3. GetTransactionHistory
- **Total**: 1 + (N × 3) calls for N retailers

### Problems
1. **N+1 Query Problem**: With 50 retailers = 151 service calls
2. **High Latency**: Even with parallelization, multiple round trips add up
3. **Service Load**: Each service gets hammered with many requests
4. **Error Cascading**: Single service failure affects all retailers
5. **No Caching**: Same data fetched repeatedly
6. **No Batching**: Can't leverage bulk operations

## Recommendations (Prioritized)

### 1. **Batch Aggregation RPCs** (High Priority) ⭐⭐⭐

Create new bulk RPCs that accept multiple retailer IDs:

```protobuf
// wallet.proto
rpc GetRetailerBalancesBatch(GetRetailerBalancesBatchRequest) returns (GetRetailerBalancesBatchResponse);

message GetRetailerBalancesBatchRequest {
  repeated string retailer_ids = 1;
  WalletType wallet_type = 2;
}

message GetRetailerBalancesBatchResponse {
  map<string, RetailerBalance> balances = 1; // key = retailer_id
}

// agent-management.proto
rpc GetPOSDeviceCountsBatch(GetPOSDeviceCountsBatchRequest) returns (GetPOSDeviceCountsBatchResponse);

message GetPOSDeviceCountsBatchRequest {
  repeated string retailer_ids = 1;
}

message GetPOSDeviceCountsBatchResponse {
  map<string, int32> counts = 1; // key = retailer_id, value = device count
}
```

**Benefits**:
- Reduces 3N calls to 3 calls total
- Single round trip per service
- Better error isolation
- Easier to cache

**Implementation**:
- Services use `WHERE retailer_id = ANY($1)` SQL pattern
- Return map for O(1) lookup

---

### 2. **Denormalize Common Fields** (High Priority) ⭐⭐⭐

Add frequently accessed fields to the `retailers` table:

```sql
ALTER TABLE retailers ADD COLUMN stake_balance BIGINT DEFAULT 0;
ALTER TABLE retailers ADD COLUMN pos_device_count INT DEFAULT 0;
ALTER TABLE retailers ADD COLUMN last_transaction_at TIMESTAMP;

-- Update via triggers or materialized view refresh
CREATE TRIGGER update_retailer_balance
  AFTER INSERT OR UPDATE ON retailer_stake_wallets
  FOR EACH ROW
  EXECUTE FUNCTION sync_retailer_balance();
```

**Benefits**:
- Zero additional calls for common fields
- Fast reads
- Can still query live data when needed

**Trade-offs**:
- Storage overhead
- Eventual consistency
- Requires sync mechanism

---

### 3. **Use Database Joins at Source** (High Priority) ⭐⭐⭐

Instead of separate service calls, create a single enriched query in `agent-management`:

```sql
-- In agent-management service
SELECT 
  r.*,
  COALESCE(rs.balance, 0) as stake_balance,
  COUNT(DISTINCT pd.id) as pos_device_count,
  MAX(wt.created_at) as last_transaction_at
FROM retailers r
LEFT JOIN retailer_stake_wallets rs ON rs.retailer_id = r.id
LEFT JOIN pos_devices pd ON pd.assigned_retailer_id = r.id
LEFT JOIN wallet_transactions wt ON wt.wallet_owner_id = r.id 
  AND wt.wallet_type = 'RETAILER_STAKE'
WHERE r.parent_agent_id = $1
GROUP BY r.id, rs.balance;
```

Add to `GetAgentRetailersResponse`:

```protobuf
message Retailer {
  // ... existing fields
  optional int64 stake_balance = 20;
  optional int32 pos_device_count = 21;
  optional google.protobuf.Timestamp last_transaction_at = 22;
}
```

**Benefits**:
- Single database query
- Atomic data
- No cross-service calls
- Fastest option

**Trade-offs**:
- Requires cross-database joins (if services have separate DBs)
- Service coupling

---

### 4. **Implement Caching Layer** (Medium Priority) ⭐⭐

Cache aggregated retailer data:

```go
// Redis cache key: "retailer:stats:{retailer_id}"
type RetailerStats struct {
  Balance            float64
  POSDeviceCount     int
  LastTransactionAt  *time.Time
  CachedAt           time.Time
}

// Cache TTL: 30-60 seconds
```

**Strategy**:
- Cache on first fetch
- Invalidate on wallet/device updates
- Use cache-aside pattern

**Benefits**:
- Dramatic latency reduction for repeated requests
- Reduces service load

**Implementation**:
```go
cacheKey := fmt.Sprintf("retailer:stats:%s", retailerID)
if cached, err := redis.Get(ctx, cacheKey); err == nil {
  return cached // Use cached data
}
// ... fetch and cache
```

---

### 5. **Two-Tier Endpoint Strategy** (Medium Priority) ⭐⭐

Separate lightweight vs detailed endpoints:

```
GET /api/v1/agent/retailers           # Basic list (no aggregation)
GET /api/v1/agent/retailers/:id/stats # Detailed stats for one
GET /api/v1/agent/retailers/stats     # Batch stats for all
```

**Benefits**:
- Fast initial load
- Progressive enhancement
- Users fetch details on demand

**Response Shape**:
```json
// Basic list
{
  "data": [
    {"id": "...", "name": "...", "status": "..."}
  ]
}

// Stats endpoint
{
  "data": [
    {"retailer_id": "...", "balance": 1234.56, "pos_count": 2}
  ]
}
```

---

### 6. **Background Pre-aggregation Jobs** (Medium Priority) ⭐⭐

Similar to your dashboard plan, create materialized views:

```sql
CREATE MATERIALIZED VIEW mv_retailer_stats AS
SELECT 
  r.id as retailer_id,
  COALESCE(rs.balance, 0) as stake_balance,
  COUNT(DISTINCT pd.id) as pos_device_count,
  MAX(wt.created_at) as last_transaction_at,
  NOW() as refreshed_at
FROM retailers r
LEFT JOIN retailer_stake_wallets rs ON rs.retailer_id = r.id
LEFT JOIN pos_devices pd ON pd.assigned_retailer_id = r.id
LEFT JOIN wallet_transactions wt ON wt.wallet_owner_id = r.id
GROUP BY r.id, rs.balance;

-- Refresh every 1-5 minutes via cron
REFRESH MATERIALIZED VIEW CONCURRENTLY mv_retailer_stats;
```

**Benefits**:
- Sub-millisecond reads
- Consistent performance
- No service load during reads

**Trade-offs**:
- Eventual consistency (1-5 min lag)
- Requires refresh infrastructure

---

### 7. **GraphQL-Style Field Selection** (Low Priority) ⭐

Allow clients to request only needed fields:

```
GET /api/v1/agent/retailers?fields=balance,pos_count
```

**Benefits**:
- Reduces data transfer
- Only fetch what's needed
- Better mobile performance

---

### 8. **Connection Pooling & Circuit Breakers** (High Priority) ⭐⭐⭐

Optimize service communication:

```go
// Connection pooling
grpc.Dial(address, 
  grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(4*1024*1024)),
  grpc.WithInitialWindowSize(65535),
  grpc.WithInitialConnWindowSize(65535),
)

// Circuit breaker
breaker := circuit.NewBreaker(circuit.Settings{
  Timeout: 5 * time.Second,
  MaxFailures: 5,
})
```

**Benefits**:
- Prevents cascading failures
- Faster recovery
- Better resource utilization

---

## Recommended Implementation Path

### Phase 1 (Quick Win) - 1-2 days
1. ✅ Add caching layer for retailer stats (Redis)
2. ✅ Add connection pooling if not present
3. ✅ Add circuit breakers

### Phase 2 (Medium Term) - 1 week
1. ✅ Create batch aggregation RPCs
2. ✅ Update `ListAgentRetailers` to use batch calls
3. ✅ Add background refresh job for materialized views

### Phase 3 (Long Term) - 2-3 weeks
1. ✅ Add enriched `GetAgentRetailers` with joins
2. ✅ Create materialized views
3. ✅ Implement two-tier endpoint strategy

## Performance Comparison

| Approach | Calls (50 retailers) | Latency | Complexity |
|----------|---------------------|---------|------------|
| **Current** | 151 | ~2-5s | Low |
| **Batch RPCs** | 4 | ~300-500ms | Medium |
| **Caching** | 0 (cache hit) | ~10-50ms | Low |
| **Database Joins** | 1 | ~50-100ms | Medium |
| **Materialized View** | 1 | ~5-20ms | High |

## Code Example: Batch Implementation

```go
// Batch fetch balances
balanceReq := &walletpb.GetRetailerBalancesBatchRequest{
    RetailerIds: retailerIDs,
    WalletType: walletpb.WalletType_RETAILER_STAKE,
}
balancesResp, _ := walletClient.GetRetailerBalancesBatch(ctx, balanceReq)

// Batch fetch POS counts
posReq := &agentmgmtpb.GetPOSDeviceCountsBatchRequest{
    RetailerIds: retailerIDs,
}
posResp, _ := amClient.GetPOSDeviceCountsBatch(ctx, posReq)

// O(1) lookup per retailer
for _, retailer := range resp.Retailers {
    retailerData["balance"] = balancesResp.Balances[retailer.Id].Balance / 100.0
    retailerData["pos_devices_count"] = posResp.Counts[retailer.Id]
}
```

## Monitoring & Metrics

Track these metrics:
- `retailer_list_latency_p50/p95/p99`
- `retailer_list_service_calls_total`
- `retailer_list_cache_hit_rate`
- `retailer_list_errors_total` (by service)

---

**Priority Ranking**:
1. **Batch RPCs** - Biggest impact, reasonable effort
2. **Caching** - Quick win, significant improvement
3. **Database Joins** - Best performance, requires coordination
4. **Materialized Views** - Ultimate performance, more complex
