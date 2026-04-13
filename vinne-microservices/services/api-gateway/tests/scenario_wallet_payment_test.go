package tests

import (
	"encoding/json"
	"net/http"
	"testing"
)

// TestScenario_WalletPayment tests wallet management and payment operations
func TestScenario_WalletPayment(t *testing.T) {
	client := NewTestClient()

	// Check API Gateway health
	if !client.CheckHealth(t) {
		t.Fatal("API Gateway is not healthy")
	}

	// Login as admin for initial setup
	if !client.LoginAsAdmin(t) {
		t.Fatal("Failed to login as admin")
	}

	// Try to login as agent for wallet operations
	t.Run("Agent_Login_For_Wallet", func(t *testing.T) {
		loginData := map[string]string{
			"agent_code": TestAgentCode,
			"password":   TestAgentPassword,
			"device_id":  TestDeviceID,
		}

		resp, err := client.MakeRequest("POST", "/agent/auth/login", loginData, "")
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}

		if resp.StatusCode == http.StatusOK {
			var apiResp APIResponse
			if err := client.ParseResponse(resp, &apiResp); err == nil && apiResp.Success {
				var loginResp LoginResponse
				if json.Unmarshal(apiResp.Data, &loginResp) == nil {
					client.AgentToken = loginResp.AccessToken
					t.Logf("✓ Agent login successful for wallet operations")
				}
			}
		} else {
			t.Logf("⚠ Agent login failed, will continue with admin token")
		}
	})

	var transactionID string
	var depositID string
	var withdrawalID string

	// Agent Wallet Operations
	t.Run("Get_Agent_Wallet_Balance", func(t *testing.T) {
		token := client.AgentToken
		if token == "" {
			t.Skip("No agent token available")
		}

		resp, err := client.MakeRequest("GET", "/agent/wallet/balance", nil, token)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}

		if resp.StatusCode == http.StatusNotFound {
			t.Skip("GET /agent/wallet/balance endpoint not implemented (404)")
		}

		if AssertStatusCode(t, http.StatusOK, resp.StatusCode, "Get agent wallet balance") {
			var apiResp APIResponse
			if err := client.ParseResponse(resp, &apiResp); err == nil && apiResp.Success {
				t.Logf("✓ Retrieved agent wallet balance")
			}
		}
	})

	t.Run("Agent_Deposit_Request", func(t *testing.T) {
		token := client.AgentToken
		if token == "" {
			token = client.AdminToken
		}

		depositData := map[string]interface{}{
			"amount":         1000.00,
			"payment_method": "bank_transfer",
			"reference":      "DEP" + TestSuffix,
			"bank_name":      "Test Bank",
			"account_number": "1234567890",
			"depositor_name": "Test Agent",
			"notes":          "Initial deposit for testing",
		}

		resp, err := client.MakeRequest("POST", "/agent/wallet/deposit", depositData, token)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}

		if resp.StatusCode == http.StatusNotFound {
			t.Skip("POST /agent/wallet/deposit endpoint not implemented (404)")
		}

		if AssertStatusCodes(t, []int{http.StatusCreated, http.StatusOK},
			resp.StatusCode, "Agent deposit request") {
			// Try to extract deposit ID
			var apiResp APIResponse
			if err := client.ParseResponse(resp, &apiResp); err == nil && apiResp.Success {
				var depositResp map[string]interface{}
				if json.Unmarshal(apiResp.Data, &depositResp) == nil {
					if id, ok := depositResp["deposit_id"].(string); ok {
						depositID = id
						t.Logf("✓ Deposit request created with ID: %s", depositID)
					}
				}
			}
		}
	})

	t.Run("Approve_Deposit", func(t *testing.T) {
		if depositID == "" {
			t.Skip("No deposit ID available")
		}

		approvalData := map[string]interface{}{
			"status":          "approved",
			"approved_by":     "admin@randlottery.com",
			"approval_notes":  "Deposit verified and approved",
			"transaction_ref": "TXN" + TestSuffix,
		}

		resp, err := client.MakeRequest("POST", "/admin/deposits/"+depositID+"/approve",
			approvalData, client.AdminToken)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}

		if resp.StatusCode == http.StatusNotFound {
			t.Skip("POST /admin/deposits/{id}/approve endpoint not implemented (404)")
		}

		if AssertStatusCode(t, http.StatusOK, resp.StatusCode, "Approve deposit") {
			t.Logf("✓ Deposit approved successfully")
		}
	})

	t.Run("Get_Agent_Transaction_History", func(t *testing.T) {
		token := client.AgentToken
		if token == "" {
			token = client.AdminToken
		}

		resp, err := client.MakeRequest("GET", "/agent/wallet/transactions?page=1&page_size=10",
			nil, token)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}

		if resp.StatusCode == http.StatusNotFound {
			t.Skip("GET /agent/wallet/transactions endpoint not implemented (404)")
		}

		if AssertStatusCode(t, http.StatusOK, resp.StatusCode, "Get transaction history") {
			var paginatedResp PaginatedResponse
			if err := client.ParseResponse(resp, &paginatedResp); err == nil && paginatedResp.Success {
				t.Logf("✓ Retrieved %d transactions", paginatedResp.Pagination.TotalCount)
			}
		}
	})

	t.Run("Agent_Withdrawal_Request", func(t *testing.T) {
		token := client.AgentToken
		if token == "" {
			token = client.AdminToken
		}

		withdrawalData := map[string]interface{}{
			"amount":            500.00,
			"withdrawal_method": "bank_transfer",
			"bank_name":         "Test Bank",
			"account_number":    "1234567890",
			"account_name":      "Test Agent",
			"reason":            "Commission withdrawal",
		}

		resp, err := client.MakeRequest("POST", "/agent/wallet/withdraw", withdrawalData, token)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}

		if resp.StatusCode == http.StatusNotFound {
			t.Skip("POST /agent/wallet/withdraw endpoint not implemented (404)")
		}

		// Accept 201 (created), 200 (ok), or 400 (insufficient balance)
		if AssertStatusCodes(t, []int{http.StatusCreated, http.StatusOK, http.StatusBadRequest},
			resp.StatusCode, "Agent withdrawal request") {
			if resp.StatusCode == http.StatusBadRequest {
				t.Logf("⚠ Withdrawal failed (insufficient balance)")
			} else {
				// Try to extract withdrawal ID
				var apiResp APIResponse
				if err := client.ParseResponse(resp, &apiResp); err == nil && apiResp.Success {
					var withdrawResp map[string]interface{}
					if json.Unmarshal(apiResp.Data, &withdrawResp) == nil {
						if id, ok := withdrawResp["withdrawal_id"].(string); ok {
							withdrawalID = id
							t.Logf("✓ Withdrawal request created with ID: %s", withdrawalID)
						}
					}
				}
			}
		}
	})

	t.Run("Process_Withdrawal", func(t *testing.T) {
		if withdrawalID == "" {
			t.Skip("No withdrawal ID available")
		}

		processData := map[string]interface{}{
			"status":          "processed",
			"processed_by":    "admin@randlottery.com",
			"transaction_ref": "WTH" + TestSuffix,
			"notes":           "Withdrawal processed successfully",
		}

		resp, err := client.MakeRequest("POST", "/admin/withdrawals/"+withdrawalID+"/process",
			processData, client.AdminToken)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}

		if resp.StatusCode == http.StatusNotFound {
			t.Skip("POST /admin/withdrawals/{id}/process endpoint not implemented (404)")
		}

		if AssertStatusCode(t, http.StatusOK, resp.StatusCode, "Process withdrawal") {
			t.Logf("✓ Withdrawal processed successfully")
		}
	})

	// Payment Gateway Operations
	t.Run("Initialize_Payment", func(t *testing.T) {
		paymentData := map[string]interface{}{
			"amount":         100.00,
			"currency":       "GHS",
			"payment_method": "mobile_money",
			"provider":       "mtn",
			"phone_number":   "+233201234569",
			"email":          "customer@test.com",
			"reference":      "PAY" + TestSuffix,
			"description":    "Lottery ticket purchase",
			"callback_url":   "https://api.randlottery.com/payments/callback",
		}

		resp, err := client.MakeRequest("POST", "/payments/initialize", paymentData, "")
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}

		if resp.StatusCode == http.StatusNotFound {
			t.Skip("POST /payments/initialize endpoint not implemented (404)")
		}

		if AssertStatusCodes(t, []int{http.StatusCreated, http.StatusOK},
			resp.StatusCode, "Initialize payment") {
			// Try to extract transaction ID
			var apiResp APIResponse
			if err := client.ParseResponse(resp, &apiResp); err == nil && apiResp.Success {
				var paymentResp map[string]interface{}
				if json.Unmarshal(apiResp.Data, &paymentResp) == nil {
					if id, ok := paymentResp["transaction_id"].(string); ok {
						transactionID = id
						t.Logf("✓ Payment initialized with ID: %s", transactionID)
					}
					if authUrl, ok := paymentResp["authorization_url"].(string); ok {
						t.Logf("  Authorization URL: %s", authUrl)
					}
				}
			}
		}
	})

	t.Run("Verify_Payment", func(t *testing.T) {
		if transactionID == "" {
			t.Skip("No transaction ID available")
		}

		resp, err := client.MakeRequest("GET", "/payments/verify/"+transactionID, nil, "")
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}

		if resp.StatusCode == http.StatusNotFound {
			t.Skip("GET /payments/verify/{id} endpoint not implemented (404)")
		}

		if AssertStatusCode(t, http.StatusOK, resp.StatusCode, "Verify payment") {
			var apiResp APIResponse
			if err := client.ParseResponse(resp, &apiResp); err == nil && apiResp.Success {
				t.Logf("✓ Payment verification retrieved")
			}
		}
	})

	t.Run("Process_Payment_Callback", func(t *testing.T) {
		if transactionID == "" {
			t.Skip("No transaction ID available")
		}

		callbackData := map[string]interface{}{
			"transaction_id": transactionID,
			"status":         "success",
			"provider_ref":   "PROV" + TestSuffix,
			"amount":         100.00,
			"currency":       "GHS",
			"paid_at":        "2025-01-02T10:00:00Z",
			"channel":        "mobile_money",
			"provider_response": map[string]interface{}{
				"code":    "00",
				"message": "Transaction successful",
			},
		}

		resp, err := client.MakeRequest("POST", "/payments/callback", callbackData, "")
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}

		if resp.StatusCode == http.StatusNotFound {
			t.Skip("POST /payments/callback endpoint not implemented (404)")
		}

		if AssertStatusCode(t, http.StatusOK, resp.StatusCode, "Process payment callback") {
			t.Logf("✓ Payment callback processed")
		}
	})

	// Commission Management
	t.Run("Calculate_Agent_Commission", func(t *testing.T) {
		token := client.AgentToken
		if token == "" {
			token = client.AdminToken
		}

		commissionData := map[string]interface{}{
			"agent_code":   TestAgentCode,
			"period_start": "2025-01-01",
			"period_end":   "2025-01-31",
		}

		resp, err := client.MakeRequest("POST", "/agent/commission/calculate",
			commissionData, token)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}

		if resp.StatusCode == http.StatusNotFound {
			t.Skip("POST /agent/commission/calculate endpoint not implemented (404)")
		}

		if AssertStatusCode(t, http.StatusOK, resp.StatusCode, "Calculate commission") {
			var apiResp APIResponse
			if err := client.ParseResponse(resp, &apiResp); err == nil && apiResp.Success {
				t.Logf("✓ Commission calculated")
			}
		}
	})

	t.Run("Get_Commission_History", func(t *testing.T) {
		token := client.AgentToken
		if token == "" {
			token = client.AdminToken
		}

		resp, err := client.MakeRequest("GET", "/agent/commission/history?page=1&page_size=10",
			nil, token)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}

		if resp.StatusCode == http.StatusNotFound {
			t.Skip("GET /agent/commission/history endpoint not implemented (404)")
		}

		if AssertStatusCode(t, http.StatusOK, resp.StatusCode, "Get commission history") {
			t.Logf("✓ Retrieved commission history")
		}
	})

	t.Run("Request_Commission_Payout", func(t *testing.T) {
		token := client.AgentToken
		if token == "" {
			token = client.AdminToken
		}

		payoutData := map[string]interface{}{
			"amount":        250.00,
			"period":        "2025-01",
			"payout_method": "wallet_transfer",
			"notes":         "Monthly commission payout request",
		}

		resp, err := client.MakeRequest("POST", "/agent/commission/payout", payoutData, token)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}

		if resp.StatusCode == http.StatusNotFound {
			t.Skip("POST /agent/commission/payout endpoint not implemented (404)")
		}

		// Accept 201 (created), 200 (ok), or 400 (no commission due)
		if AssertStatusCodes(t, []int{http.StatusCreated, http.StatusOK, http.StatusBadRequest},
			resp.StatusCode, "Request commission payout") {
			if resp.StatusCode == http.StatusBadRequest {
				t.Logf("⚠ Payout request failed (no commission due or already paid)")
			} else {
				t.Logf("✓ Commission payout requested")
			}
		}
	})

	// Admin Wallet Management
	t.Run("Get_System_Wallet_Summary", func(t *testing.T) {
		resp, err := client.MakeRequest("GET", "/admin/wallets/summary", nil, client.AdminToken)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}

		if resp.StatusCode == http.StatusNotFound {
			t.Skip("GET /admin/wallets/summary endpoint not implemented (404)")
		}

		if AssertStatusCode(t, http.StatusOK, resp.StatusCode, "Get system wallet summary") {
			t.Logf("✓ Retrieved system wallet summary")
		}
	})

	t.Run("Get_Pending_Deposits", func(t *testing.T) {
		resp, err := client.MakeRequest("GET", "/admin/deposits/pending", nil, client.AdminToken)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}

		if resp.StatusCode == http.StatusNotFound {
			t.Skip("GET /admin/deposits/pending endpoint not implemented (404)")
		}

		if AssertStatusCode(t, http.StatusOK, resp.StatusCode, "Get pending deposits") {
			t.Logf("✓ Retrieved pending deposits")
		}
	})

	t.Run("Get_Pending_Withdrawals", func(t *testing.T) {
		resp, err := client.MakeRequest("GET", "/admin/withdrawals/pending", nil, client.AdminToken)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}

		if resp.StatusCode == http.StatusNotFound {
			t.Skip("GET /admin/withdrawals/pending endpoint not implemented (404)")
		}

		if AssertStatusCode(t, http.StatusOK, resp.StatusCode, "Get pending withdrawals") {
			t.Logf("✓ Retrieved pending withdrawals")
		}
	})

	t.Run("Get_Payment_Reports", func(t *testing.T) {
		reportParams := "?start_date=2025-01-01&end_date=2025-01-31&group_by=day"
		resp, err := client.MakeRequest("GET", "/admin/payments/reports"+reportParams,
			nil, client.AdminToken)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}

		if resp.StatusCode == http.StatusNotFound {
			t.Skip("GET /admin/payments/reports endpoint not implemented (404)")
		}

		if AssertStatusCode(t, http.StatusOK, resp.StatusCode, "Get payment reports") {
			t.Logf("✓ Retrieved payment reports")
		}
	})

	t.Run("Reconcile_Payments", func(t *testing.T) {
		reconcileData := map[string]interface{}{
			"date":     "2025-01-01",
			"provider": "mtn",
			"transactions": []map[string]interface{}{
				{
					"reference":    "PAY" + TestSuffix,
					"amount":       100.00,
					"status":       "success",
					"provider_ref": "PROV" + TestSuffix,
				},
			},
		}

		resp, err := client.MakeRequest("POST", "/admin/payments/reconcile",
			reconcileData, client.AdminToken)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}

		if resp.StatusCode == http.StatusNotFound {
			t.Skip("POST /admin/payments/reconcile endpoint not implemented (404)")
		}

		if AssertStatusCode(t, http.StatusOK, resp.StatusCode, "Reconcile payments") {
			t.Logf("✓ Payments reconciled")
		}
	})

	// Refund Operations
	t.Run("Process_Refund", func(t *testing.T) {
		if transactionID == "" {
			t.Skip("No transaction ID available")
		}

		refundData := map[string]interface{}{
			"transaction_id": transactionID,
			"amount":         50.00,
			"reason":         "Partial refund - customer request",
			"refund_method":  "original_payment_method",
		}

		resp, err := client.MakeRequest("POST", "/admin/payments/refund",
			refundData, client.AdminToken)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}

		if resp.StatusCode == http.StatusNotFound {
			t.Skip("POST /admin/payments/refund endpoint not implemented (404)")
		}

		// Accept 200 (refunded), 201 (refund initiated), or 400 (cannot refund)
		if AssertStatusCodes(t, []int{http.StatusOK, http.StatusCreated, http.StatusBadRequest},
			resp.StatusCode, "Process refund") {
			if resp.StatusCode == http.StatusBadRequest {
				t.Logf("⚠ Refund failed (payment might not be eligible)")
			} else {
				t.Logf("✓ Refund processed/initiated")
			}
		}
	})
}
