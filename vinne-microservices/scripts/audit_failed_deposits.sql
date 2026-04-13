-- Audit Script for Failed Player Deposit Transactions
-- This script identifies player deposit transactions that failed but may have
-- incorrectly credited player wallets due to the saga compensation bug.
--
-- Run this script against both payment_service and wallet_service databases
-- to identify discrepancies and wallet balances that need correction.
--
-- Author: System
-- Date: 2025-10-31
-- Related Issue: Failed transactions showing credited wallet balances

-- ============================================================================
-- PART 1: Query Payment Service Database
-- ============================================================================
-- Run these queries against the payment_service database

\echo '========================================';
\echo 'PAYMENT SERVICE - Failed Transactions';
\echo '========================================';
\echo '';

-- Find all failed/compensated player deposit sagas in the last 7 days
\echo 'Failed Player Deposit Sagas (Last 7 Days):';
SELECT
    s.saga_id,
    s.transaction_id,
    s.status,
    s.current_step,
    s.total_steps,
    s.created_at,
    s.updated_at,
    s.compensation_data
FROM sagas s
WHERE s.saga_id LIKE 'player-deposit-%'
  AND s.status IN ('COMPENSATED', 'FAILED')
  AND s.created_at >= NOW() - INTERVAL '7 days'
ORDER BY s.created_at DESC;

\echo '';
\echo 'Saga Steps for Failed Transactions:';
-- Get detailed steps for failed sagas
SELECT
    s.saga_id,
    ss.step_index,
    ss.step_name,
    ss.direction,
    ss.status,
    ss.error_message,
    ss.executed_at,
    ss.completed_at
FROM sagas s
JOIN saga_steps ss ON ss.saga_id = s.id
WHERE s.saga_id LIKE 'player-deposit-%'
  AND s.status IN ('COMPENSATED', 'FAILED')
  AND s.created_at >= NOW() - INTERVAL '7 days'
ORDER BY s.created_at DESC, ss.step_index;

\echo '';
\echo 'Payment Transactions with NULL provider_transaction_id:';
-- Find transactions that failed with NULL provider_transaction_id error
SELECT
    t.id,
    t.reference,
    t.provider_transaction_id,
    t.type,
    t.status,
    t.amount,
    t.currency,
    t.user_id,
    t.error_message,
    t.created_at,
    t.updated_at
FROM transactions t
WHERE t.type = 'DEPOSIT'
  AND t.status IN ('FAILED', 'PENDING')
  AND t.created_at >= NOW() - INTERVAL '7 days'
ORDER BY t.created_at DESC;

-- ============================================================================
-- PART 2: Query Wallet Service Database
-- ============================================================================
-- Run these queries against the wallet_service database

\echo '';
\echo '========================================';
\echo 'WALLET SERVICE - Potential Issues';
\echo '========================================';
\echo '';

-- Find player wallet credits that may need reversal
\echo 'Player Wallet Credits Without Matching Reversals:';
SELECT
    wt.id,
    wt.transaction_id,
    wt.wallet_owner_id AS player_id,
    wt.transaction_type,
    wt.amount,
    wt.balance_before,
    wt.balance_after,
    wt.description,
    wt.status,
    wt.idempotency_key,
    wt.is_reversed,
    wt.reversed_by_transaction_id,
    wt.created_at
FROM wallet_transactions wt
WHERE wt.wallet_type = 'PLAYER_WALLET'
  AND wt.transaction_type = 'CREDIT'
  AND wt.status = 'COMPLETED'
  AND wt.is_reversed = FALSE
  AND wt.idempotency_key LIKE 'REF-%'
  AND wt.created_at >= NOW() - INTERVAL '7 days'
  -- Exclude credits that have a corresponding debit with -reverse suffix
  AND NOT EXISTS (
    SELECT 1 FROM wallet_transactions wt2
    WHERE wt2.wallet_owner_id = wt.wallet_owner_id
      AND wt2.transaction_type = 'DEBIT'
      AND wt2.idempotency_key = wt.idempotency_key || '-reverse'
      AND wt2.status = 'COMPLETED'
  )
ORDER BY wt.created_at DESC;

\echo '';
\echo 'Player Wallet Credits with Failed Reversals:';
-- Find credits with reversal attempts that failed
SELECT
    wt_credit.transaction_id AS credit_txn,
    wt_debit.transaction_id AS debit_txn,
    wt_credit.wallet_owner_id AS player_id,
    wt_credit.amount AS credit_amount,
    wt_credit.idempotency_key AS credit_key,
    wt_debit.idempotency_key AS debit_key,
    wt_debit.status AS debit_status,
    wt_credit.created_at AS credit_at,
    wt_debit.created_at AS debit_at
FROM wallet_transactions wt_credit
LEFT JOIN wallet_transactions wt_debit
    ON wt_debit.wallet_owner_id = wt_credit.wallet_owner_id
    AND wt_debit.transaction_type = 'DEBIT'
    AND wt_debit.idempotency_key = wt_credit.idempotency_key || '-reverse'
WHERE wt_credit.wallet_type = 'PLAYER_WALLET'
  AND wt_credit.transaction_type = 'CREDIT'
  AND wt_credit.status = 'COMPLETED'
  AND wt_credit.idempotency_key LIKE 'REF-%'
  AND wt_credit.created_at >= NOW() - INTERVAL '7 days'
  AND wt_debit.status IN ('FAILED', 'PENDING')
ORDER BY wt_credit.created_at DESC;

\echo '';
\echo 'Audit Summary:';
-- Summary of potentially affected wallets
SELECT
    COUNT(*) AS affected_credits,
    SUM(wt.amount) AS total_amount_credited,
    COUNT(DISTINCT wt.wallet_owner_id) AS affected_players
FROM wallet_transactions wt
WHERE wt.wallet_type = 'PLAYER_WALLET'
  AND wt.transaction_type = 'CREDIT'
  AND wt.status = 'COMPLETED'
  AND wt.is_reversed = FALSE
  AND wt.idempotency_key LIKE 'REF-%'
  AND wt.created_at >= NOW() - INTERVAL '7 days'
  AND NOT EXISTS (
    SELECT 1 FROM wallet_transactions wt2
    WHERE wt2.wallet_owner_id = wt.wallet_owner_id
      AND wt2.transaction_type = 'DEBIT'
      AND wt2.idempotency_key = wt.idempotency_key || '-reverse'
      AND wt2.status = 'COMPLETED'
  );

-- ============================================================================
-- PART 3: Manual Correction Steps
-- ============================================================================
\echo '';
\echo '========================================';
\echo 'MANUAL CORRECTION STEPS';
\echo '========================================';
\echo '';
\echo 'For each affected transaction identified above:';
\echo '';
\echo '1. Verify the transaction in both payment and wallet databases';
\echo '2. Check if the mobile money debit was actually charged (check with Orange API)';
\echo '3. If charged and wallet was credited: Transaction is valid (no action needed)';
\echo '4. If NOT charged but wallet was credited: Need to reverse the credit';
\echo '5. If charged but wallet was NOT credited: Need to manually credit (rare case)';
\echo '';
\echo 'To manually reverse a wallet credit (if needed):';
\echo 'Use the ReverseTransaction gRPC method in wallet service:';
\echo '   grpcurl -d ''{"transaction_id": "<uuid>", "reason": "Manual reversal - failed deposit", "admin_id": "<admin_uuid>", "admin_name": "System Admin", "admin_email": "admin@randlottery.com"}'' localhost:50059 wallet.v1.WalletService/ReverseTransaction';
\echo '';
\echo 'IMPORTANT: Always verify with Orange API status before reversing!';
\echo '';

-- ============================================================================
-- PART 4: Prevention Check
-- ============================================================================
\echo '========================================';
\echo 'PREVENTION VERIFICATION';
\echo '========================================';
\echo '';
\echo 'After deploying the fix, verify that new transactions handle errors correctly:';
\echo '1. Check that provider_transaction_id is nullable in payment service';
\echo '2. Verify saga compensation skips debit when wallet_transaction_id is missing';
\echo '3. Monitor logs for "Skipping wallet debit compensation" messages';
\echo '4. Test with a failed deposit scenario to confirm proper rollback';
\echo '';
