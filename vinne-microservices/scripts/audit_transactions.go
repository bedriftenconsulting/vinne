// Standalone utility script for auditing failed deposit transactions
// Run with: go run audit_transactions.go -payment-db=<dsn> -wallet-db=<dsn>
//
//go:build standalone
// +build standalone

package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"os"
	"time"

	_ "github.com/lib/pq"
)

// AuditResult represents the result of the audit
type AuditResult struct {
	TotalFailedSagas       int
	TotalOrphanedCredits   int
	TotalAffectedPlayers   int
	TotalAmountCredited    int64
	OrphanedTransactions   []OrphanedTransaction
	FailedReversalAttempts []FailedReversal
}

// OrphanedTransaction represents a wallet credit without proper reversal
type OrphanedTransaction struct {
	ID             string
	TransactionID  string
	PlayerID       string
	Amount         int64
	IdempotencyKey string
	CreatedAt      time.Time
	Description    string
	BalanceBefore  int64
	BalanceAfter   int64
}

// FailedReversal represents a reversal attempt that failed
type FailedReversal struct {
	CreditTransactionID string
	DebitTransactionID  string
	PlayerID            string
	Amount              int64
	DebitStatus         string
	CreditKey           string
	DebitKey            string
	CreditCreatedAt     time.Time
	DebitCreatedAt      time.Time
}

func main() {
	// Parse command line flags
	paymentDSN := flag.String("payment-db", "", "Payment service database connection string")
	walletDSN := flag.String("wallet-db", "", "Wallet service database connection string")
	days := flag.Int("days", 7, "Number of days to look back")
	verbose := flag.Bool("verbose", false, "Enable verbose output")

	flag.Parse()

	// Check required flags
	if *paymentDSN == "" || *walletDSN == "" {
		fmt.Println("Usage: go run audit_transactions.go -payment-db=<dsn> -wallet-db=<dsn> [-days=7] [-verbose]")
		fmt.Println("")
		fmt.Println("Example:")
		fmt.Println("  go run audit_transactions.go \\")
		fmt.Println("    -payment-db='postgresql://payment:Password@localhost:5432/payment_service?sslmode=disable' \\")
		fmt.Println("    -wallet-db='postgresql://wallet:Password@localhost:5432/wallet_service?sslmode=disable' \\")
		fmt.Println("    -days=7 -verbose")
		os.Exit(1)
	}

	ctx := context.Background()

	fmt.Println("========================================")
	fmt.Println("Transaction Audit Tool")
	fmt.Println("========================================")
	fmt.Printf("Audit Period: Last %d days\n", *days)
	fmt.Println("")

	// Connect to payment database
	paymentDB, err := sql.Open("postgres", *paymentDSN)
	if err != nil {
		fmt.Printf("Error connecting to payment database: %v\n", err)
		os.Exit(1)
	}
	defer func() {
		_ = paymentDB.Close()
	}()

	// Connect to wallet database
	walletDB, err := sql.Open("postgres", *walletDSN)
	if err != nil {
		fmt.Printf("Error connecting to wallet database: %v\n", err)
		os.Exit(1)
	}
	defer func() {
		_ = walletDB.Close()
	}()

	// Verify connections
	if err := paymentDB.Ping(); err != nil {
		fmt.Printf("Error pinging payment database: %v\n", err)
		os.Exit(1)
	}
	if err := walletDB.Ping(); err != nil {
		fmt.Printf("Error pinging wallet database: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("✓ Database connections established")
	fmt.Println("")

	// Run audit
	result, err := runAudit(ctx, paymentDB, walletDB, *days, *verbose)
	if err != nil {
		fmt.Printf("Audit failed: %v\n", err)
		os.Exit(1)
	}

	// Print results
	printAuditResults(result, *verbose)
}

func runAudit(ctx context.Context, paymentDB, walletDB *sql.DB, days int, verbose bool) (*AuditResult, error) {
	result := &AuditResult{
		OrphanedTransactions:   make([]OrphanedTransaction, 0),
		FailedReversalAttempts: make([]FailedReversal, 0),
	}

	// 1. Count failed sagas in payment service
	if verbose {
		fmt.Println("Querying failed sagas...")
	}
	failedSagasQuery := `
		SELECT COUNT(*)
		FROM sagas
		WHERE saga_id LIKE 'player-deposit-%'
		  AND status IN ('COMPENSATED', 'FAILED')
		  AND created_at >= NOW() - $1::interval
	`
	err := paymentDB.QueryRowContext(ctx, failedSagasQuery, fmt.Sprintf("%d days", days)).Scan(&result.TotalFailedSagas)
	if err != nil {
		return nil, fmt.Errorf("failed to count failed sagas: %w", err)
	}

	// 2. Find orphaned wallet credits (credits without matching reversals)
	if verbose {
		fmt.Println("Searching for orphaned wallet credits...")
	}
	orphanedQuery := `
		SELECT
			wt.id,
			wt.transaction_id,
			wt.wallet_owner_id,
			wt.amount,
			COALESCE(wt.idempotency_key, ''),
			wt.created_at,
			COALESCE(wt.description, ''),
			wt.balance_before,
			wt.balance_after
		FROM wallet_transactions wt
		WHERE wt.wallet_type = 'PLAYER_WALLET'
		  AND wt.transaction_type = 'CREDIT'
		  AND wt.status = 'COMPLETED'
		  AND wt.is_reversed = FALSE
		  AND wt.idempotency_key LIKE 'REF-%'
		  AND wt.created_at >= NOW() - $1::interval
		  AND NOT EXISTS (
			SELECT 1 FROM wallet_transactions wt2
			WHERE wt2.wallet_owner_id = wt.wallet_owner_id
			  AND wt2.transaction_type = 'DEBIT'
			  AND wt2.idempotency_key = wt.idempotency_key || '-reverse'
			  AND wt2.status = 'COMPLETED'
		  )
		ORDER BY wt.created_at DESC
	`

	rows, err := walletDB.QueryContext(ctx, orphanedQuery, fmt.Sprintf("%d days", days))
	if err != nil {
		return nil, fmt.Errorf("failed to query orphaned credits: %w", err)
	}
	defer func() {
		_ = rows.Close()
	}()

	for rows.Next() {
		var tx OrphanedTransaction
		var desc sql.NullString
		err := rows.Scan(
			&tx.ID,
			&tx.TransactionID,
			&tx.PlayerID,
			&tx.Amount,
			&tx.IdempotencyKey,
			&tx.CreatedAt,
			&desc,
			&tx.BalanceBefore,
			&tx.BalanceAfter,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan orphaned transaction: %w", err)
		}
		if desc.Valid {
			tx.Description = desc.String
		}
		result.OrphanedTransactions = append(result.OrphanedTransactions, tx)
		result.TotalAmountCredited += tx.Amount
	}

	result.TotalOrphanedCredits = len(result.OrphanedTransactions)

	// 3. Find failed reversal attempts
	if verbose {
		fmt.Println("Searching for failed reversal attempts...")
	}
	failedReversalsQuery := `
		SELECT
			wt_credit.transaction_id,
			COALESCE(wt_debit.transaction_id, ''),
			wt_credit.wallet_owner_id,
			wt_credit.amount,
			COALESCE(wt_debit.status, 'NOT_FOUND'),
			COALESCE(wt_credit.idempotency_key, ''),
			COALESCE(wt_debit.idempotency_key, ''),
			wt_credit.created_at,
			COALESCE(wt_debit.created_at, wt_credit.created_at)
		FROM wallet_transactions wt_credit
		LEFT JOIN wallet_transactions wt_debit
			ON wt_debit.wallet_owner_id = wt_credit.wallet_owner_id
			AND wt_debit.transaction_type = 'DEBIT'
			AND wt_debit.idempotency_key = wt_credit.idempotency_key || '-reverse'
		WHERE wt_credit.wallet_type = 'PLAYER_WALLET'
		  AND wt_credit.transaction_type = 'CREDIT'
		  AND wt_credit.status = 'COMPLETED'
		  AND wt_credit.idempotency_key LIKE 'REF-%'
		  AND wt_credit.created_at >= NOW() - $1::interval
		  AND (wt_debit.status IN ('FAILED', 'PENDING') OR wt_debit.status IS NULL)
		ORDER BY wt_credit.created_at DESC
	`

	rows2, err := walletDB.QueryContext(ctx, failedReversalsQuery, fmt.Sprintf("%d days", days))
	if err != nil {
		return nil, fmt.Errorf("failed to query failed reversals: %w", err)
	}
	defer func() {
		_ = rows2.Close()
	}()

	playerMap := make(map[string]bool)
	for rows2.Next() {
		var rev FailedReversal
		err := rows2.Scan(
			&rev.CreditTransactionID,
			&rev.DebitTransactionID,
			&rev.PlayerID,
			&rev.Amount,
			&rev.DebitStatus,
			&rev.CreditKey,
			&rev.DebitKey,
			&rev.CreditCreatedAt,
			&rev.DebitCreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan failed reversal: %w", err)
		}
		result.FailedReversalAttempts = append(result.FailedReversalAttempts, rev)
		playerMap[rev.PlayerID] = true
	}

	result.TotalAffectedPlayers = len(playerMap)

	return result, nil
}

func printAuditResults(result *AuditResult, verbose bool) {
	fmt.Println("========================================")
	fmt.Println("AUDIT RESULTS")
	fmt.Println("========================================")
	fmt.Println("")

	// Summary
	fmt.Println("Summary:")
	fmt.Printf("  Failed Sagas: %d\n", result.TotalFailedSagas)
	fmt.Printf("  Orphaned Credits: %d\n", result.TotalOrphanedCredits)
	fmt.Printf("  Total Amount Incorrectly Credited: GHS %.2f\n", float64(result.TotalAmountCredited)/100.0)
	fmt.Printf("  Affected Players: %d\n", result.TotalAffectedPlayers)
	fmt.Println("")

	// Orphaned transactions
	if result.TotalOrphanedCredits > 0 {
		fmt.Println("⚠️  ORPHANED WALLET CREDITS FOUND")
		fmt.Println("These transactions were credited but never reversed:")
		fmt.Println("")

		if verbose {
			for i, tx := range result.OrphanedTransactions {
				fmt.Printf("%d. Transaction: %s\n", i+1, tx.TransactionID)
				fmt.Printf("   Player ID: %s\n", tx.PlayerID)
				fmt.Printf("   Amount: GHS %.2f\n", float64(tx.Amount)/100.0)
				fmt.Printf("   Idempotency Key: %s\n", tx.IdempotencyKey)
				fmt.Printf("   Created At: %s\n", tx.CreatedAt.Format(time.RFC3339))
				fmt.Printf("   Description: %s\n", tx.Description)
				fmt.Printf("   Balance: %.2f → %.2f\n", float64(tx.BalanceBefore)/100.0, float64(tx.BalanceAfter)/100.0)
				fmt.Println("")
			}
		} else {
			fmt.Println("   Run with -verbose flag to see detailed transaction list")
			fmt.Println("")
		}
	} else {
		fmt.Println("✓ No orphaned wallet credits found")
		fmt.Println("")
	}

	// Failed reversals
	if len(result.FailedReversalAttempts) > 0 {
		fmt.Println("⚠️  FAILED REVERSAL ATTEMPTS FOUND")
		fmt.Println("These credits have failed reversal attempts:")
		fmt.Println("")

		if verbose {
			for i, rev := range result.FailedReversalAttempts {
				fmt.Printf("%d. Credit Transaction: %s\n", i+1, rev.CreditTransactionID)
				fmt.Printf("   Debit Transaction: %s\n", rev.DebitTransactionID)
				fmt.Printf("   Player ID: %s\n", rev.PlayerID)
				fmt.Printf("   Amount: GHS %.2f\n", float64(rev.Amount)/100.0)
				fmt.Printf("   Debit Status: %s\n", rev.DebitStatus)
				fmt.Printf("   Credit Key: %s\n", rev.CreditKey)
				fmt.Printf("   Debit Key: %s\n", rev.DebitKey)
				fmt.Println("")
			}
		} else {
			fmt.Println("   Run with -verbose flag to see detailed list")
			fmt.Println("")
		}
	} else {
		fmt.Println("✓ No failed reversal attempts found")
		fmt.Println("")
	}

	// Recommendations
	fmt.Println("========================================")
	fmt.Println("RECOMMENDATIONS")
	fmt.Println("========================================")
	fmt.Println("")

	if result.TotalOrphanedCredits > 0 || len(result.FailedReversalAttempts) > 0 {
		fmt.Println("Action Required:")
		fmt.Println("1. For each affected transaction, verify with Orange API if the mobile money debit succeeded")
		fmt.Println("2. If debit DID succeed: No action needed (valid transaction)")
		fmt.Println("3. If debit did NOT succeed: Reverse the wallet credit using ReverseTransaction gRPC method")
		fmt.Println("")
		fmt.Println("Manual Reversal Command (example):")
		fmt.Println("  grpcurl -d '{\"transaction_id\": \"<uuid>\", \"reason\": \"Manual reversal - failed deposit\", \"admin_id\": \"<admin_uuid>\", \"admin_name\": \"System Admin\", \"admin_email\": \"admin@randlottery.com\"}' localhost:50059 wallet.v1.WalletService/ReverseTransaction")
		fmt.Println("")
		fmt.Println("⚠️  IMPORTANT: Always verify with Orange API before reversing!")
	} else {
		fmt.Println("✓ No issues found. All transactions are properly handled.")
	}
	fmt.Println("")
}
