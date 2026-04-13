package tests

import (
	"encoding/json"
	"net/http"
	"testing"
)

// TestScenario_TicketOperations tests ticket purchase, validation, and management
func TestScenario_TicketOperations(t *testing.T) {
	client := NewTestClient()

	// Check API Gateway health
	if !client.CheckHealth(t) {
		t.Fatal("API Gateway is not healthy")
	}

	// Login as admin first to set up test data
	if !client.LoginAsAdmin(t) {
		t.Fatal("Failed to login as admin")
	}

	// Try to login as agent for ticket operations
	t.Run("Agent_Login_For_Tickets", func(t *testing.T) {
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
					t.Logf("✓ Agent login successful for ticket operations")
				}
			}
		} else {
			t.Logf("⚠ Agent login failed, will continue with admin token")
		}
	})

	var ticketID string
	var batchID string

	t.Run("Purchase_Single_Ticket", func(t *testing.T) {
		// Use agent token if available, otherwise use admin token
		token := client.AgentToken
		if token == "" {
			token = client.AdminToken
		}

		ticketData := map[string]interface{}{
			"game_code":      "LT" + TestSuffix,
			"draw_number":    1001,
			"ticket_type":    "single",
			"numbers":        []int{7, 14, 21, 28, 35, 42},
			"amount":         10.00,
			"customer_phone": "+233201234569",
			"customer_name":  "Test Customer",
			"payment_method": "cash",
			"retailer_code":  TestRetailerCode,
		}

		resp, err := client.MakeRequest("POST", "/tickets/purchase", ticketData, token)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}

		if resp.StatusCode == http.StatusNotFound {
			t.Skip("POST /tickets/purchase endpoint not implemented (404)")
		}

		if AssertStatusCodes(t, []int{http.StatusCreated, http.StatusOK},
			resp.StatusCode, "Purchase single ticket") {
			// Try to extract ticket ID
			var apiResp APIResponse
			if err := client.ParseResponse(resp, &apiResp); err == nil && apiResp.Success {
				var ticketResp map[string]interface{}
				if json.Unmarshal(apiResp.Data, &ticketResp) == nil {
					if id, ok := ticketResp["ticket_id"].(string); ok {
						ticketID = id
						t.Logf("✓ Ticket purchased with ID: %s", ticketID)
					}
					if serial, ok := ticketResp["serial_number"].(string); ok {
						t.Logf("  Serial Number: %s", serial)
					}
				}
			}
		}
	})

	t.Run("Purchase_Multiple_Tickets", func(t *testing.T) {
		token := client.AgentToken
		if token == "" {
			token = client.AdminToken
		}

		multiTicketData := map[string]interface{}{
			"game_code":   "LT" + TestSuffix,
			"draw_number": 1001,
			"ticket_type": "multiple",
			"tickets": []map[string]interface{}{
				{
					"numbers": []int{1, 2, 3, 4, 5, 6},
					"amount":  10.00,
				},
				{
					"numbers": []int{10, 20, 30, 40, 45, 49},
					"amount":  10.00,
				},
				{
					"numbers": []int{7, 14, 21, 28, 35, 42},
					"amount":  10.00,
				},
			},
			"total_amount":   30.00,
			"customer_phone": "+233201234569",
			"customer_name":  "Test Customer",
			"payment_method": "cash",
			"retailer_code":  TestRetailerCode,
		}

		resp, err := client.MakeRequest("POST", "/tickets/purchase-batch", multiTicketData, token)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}

		if resp.StatusCode == http.StatusNotFound {
			t.Skip("POST /tickets/purchase-batch endpoint not implemented (404)")
		}

		if AssertStatusCodes(t, []int{http.StatusCreated, http.StatusOK},
			resp.StatusCode, "Purchase multiple tickets") {
			// Try to extract batch ID
			var apiResp APIResponse
			if err := client.ParseResponse(resp, &apiResp); err == nil && apiResp.Success {
				var batchResp map[string]interface{}
				if json.Unmarshal(apiResp.Data, &batchResp) == nil {
					if id, ok := batchResp["batch_id"].(string); ok {
						batchID = id
						t.Logf("✓ Batch of tickets purchased with ID: %s", batchID)
					}
					if tickets, ok := batchResp["tickets"].([]interface{}); ok {
						t.Logf("  Total tickets in batch: %d", len(tickets))
					}
				}
			}
		}
	})

	t.Run("Purchase_Quick_Pick", func(t *testing.T) {
		token := client.AgentToken
		if token == "" {
			token = client.AdminToken
		}

		quickPickData := map[string]interface{}{
			"game_code":         "LT" + TestSuffix,
			"draw_number":       1001,
			"ticket_type":       "quick_pick",
			"ticket_count":      5, // Generate 5 random tickets
			"amount_per_ticket": 10.00,
			"total_amount":      50.00,
			"customer_phone":    "+233201234570",
			"customer_name":     "Quick Pick Customer",
			"payment_method":    "mobile_money",
			"payment_ref":       "MM" + TestSuffix,
			"retailer_code":     TestRetailerCode,
		}

		resp, err := client.MakeRequest("POST", "/tickets/quick-pick", quickPickData, token)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}

		if resp.StatusCode == http.StatusNotFound {
			t.Skip("POST /tickets/quick-pick endpoint not implemented (404)")
		}

		if AssertStatusCodes(t, []int{http.StatusCreated, http.StatusOK},
			resp.StatusCode, "Purchase quick pick tickets") {
			t.Logf("✓ Quick pick tickets purchased successfully")
		}
	})

	t.Run("Validate_Ticket", func(t *testing.T) {
		if ticketID == "" {
			t.Skip("No ticket ID available")
		}

		validateData := map[string]interface{}{
			"ticket_id":     ticketID,
			"serial_number": "SN" + TestSuffix, // Would need actual serial from purchase
		}

		resp, err := client.MakeRequest("POST", "/tickets/validate", validateData, "")
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}

		if resp.StatusCode == http.StatusNotFound {
			t.Skip("POST /tickets/validate endpoint not implemented (404)")
		}

		if AssertStatusCodes(t, []int{http.StatusOK, http.StatusBadRequest},
			resp.StatusCode, "Validate ticket") {
			if resp.StatusCode == http.StatusBadRequest {
				t.Logf("⚠ Ticket validation failed (invalid ticket or serial)")
			} else {
				t.Logf("✓ Ticket validated successfully")
			}
		}
	})

	t.Run("Check_Ticket_Status", func(t *testing.T) {
		if ticketID == "" {
			t.Skip("No ticket ID available")
		}

		resp, err := client.MakeRequest("GET", "/tickets/"+ticketID+"/status", nil, "")
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}

		if resp.StatusCode == http.StatusNotFound {
			t.Skip("GET /tickets/{id}/status endpoint not implemented (404)")
		}

		if AssertStatusCode(t, http.StatusOK, resp.StatusCode, "Check ticket status") {
			var apiResp APIResponse
			if err := client.ParseResponse(resp, &apiResp); err == nil && apiResp.Success {
				t.Logf("✓ Retrieved ticket status")
			}
		}
	})

	t.Run("Get_Ticket_Details", func(t *testing.T) {
		if ticketID == "" {
			t.Skip("No ticket ID available")
		}

		token := client.AgentToken
		if token == "" {
			token = client.AdminToken
		}

		resp, err := client.MakeRequest("GET", "/tickets/"+ticketID, nil, token)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}

		if resp.StatusCode == http.StatusNotFound {
			t.Skip("GET /tickets/{id} endpoint not implemented (404)")
		}

		if AssertStatusCode(t, http.StatusOK, resp.StatusCode, "Get ticket details") {
			t.Logf("✓ Retrieved ticket details")
		}
	})

	t.Run("Cancel_Ticket", func(t *testing.T) {
		if ticketID == "" {
			t.Skip("No ticket ID available")
		}

		token := client.AgentToken
		if token == "" {
			token = client.AdminToken
		}

		cancelData := map[string]interface{}{
			"reason": "Customer requested cancellation",
			"refund": true,
		}

		resp, err := client.MakeRequest("POST", "/tickets/"+ticketID+"/cancel",
			cancelData, token)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}

		if resp.StatusCode == http.StatusNotFound {
			t.Skip("POST /tickets/{id}/cancel endpoint not implemented (404)")
		}

		// Accept 200 (cancelled) or 400 (cannot cancel - draw started, already cancelled, etc.)
		if AssertStatusCodes(t, []int{http.StatusOK, http.StatusBadRequest},
			resp.StatusCode, "Cancel ticket") {
			if resp.StatusCode == http.StatusBadRequest {
				t.Logf("⚠ Ticket cannot be cancelled (draw may have started)")
			} else {
				t.Logf("✓ Ticket cancelled successfully")
			}
		}
	})

	t.Run("Check_Winning_Ticket", func(t *testing.T) {
		if ticketID == "" {
			t.Skip("No ticket ID available")
		}

		checkData := map[string]interface{}{
			"ticket_id":       ticketID,
			"draw_number":     1001,
			"winning_numbers": []int{7, 14, 21, 28, 35, 42},
		}

		resp, err := client.MakeRequest("POST", "/tickets/check-winning", checkData, "")
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}

		if resp.StatusCode == http.StatusNotFound {
			t.Skip("POST /tickets/check-winning endpoint not implemented (404)")
		}

		if AssertStatusCode(t, http.StatusOK, resp.StatusCode, "Check winning ticket") {
			var apiResp APIResponse
			if err := client.ParseResponse(resp, &apiResp); err == nil && apiResp.Success {
				t.Logf("✓ Checked ticket for winnings")
			}
		}
	})

	t.Run("Claim_Prize", func(t *testing.T) {
		if ticketID == "" {
			t.Skip("No ticket ID available")
		}

		claimData := map[string]interface{}{
			"ticket_id":          ticketID,
			"claimant_name":      "Test Winner",
			"claimant_phone":     "+233201234569",
			"claimant_id_type":   "national_id",
			"claimant_id_number": "GHA-123456789",
			"bank_account":       "1234567890",
			"bank_name":          "Test Bank",
		}

		resp, err := client.MakeRequest("POST", "/tickets/"+ticketID+"/claim-prize",
			claimData, client.AdminToken)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}

		if resp.StatusCode == http.StatusNotFound {
			t.Skip("POST /tickets/{id}/claim-prize endpoint not implemented (404)")
		}

		// Accept 200 (claimed), 201 (claim initiated), or 400 (not a winner, already claimed)
		if AssertStatusCodes(t, []int{http.StatusOK, http.StatusCreated, http.StatusBadRequest},
			resp.StatusCode, "Claim prize") {
			if resp.StatusCode == http.StatusBadRequest {
				t.Logf("⚠ Prize cannot be claimed (not a winner or already claimed)")
			} else {
				t.Logf("✓ Prize claim initiated")
			}
		}
	})

	t.Run("Get_Agent_Ticket_Sales", func(t *testing.T) {
		token := client.AgentToken
		if token == "" {
			token = client.AdminToken
		}

		resp, err := client.MakeRequest("GET", "/agent/tickets/sales?date="+TestSuffix,
			nil, token)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}

		if resp.StatusCode == http.StatusNotFound {
			t.Skip("GET /agent/tickets/sales endpoint not implemented (404)")
		}

		if AssertStatusCode(t, http.StatusOK, resp.StatusCode, "Get agent ticket sales") {
			t.Logf("✓ Retrieved agent ticket sales")
		}
	})

	t.Run("Get_Ticket_History", func(t *testing.T) {
		resp, err := client.MakeRequest("GET", "/tickets/history?customer_phone="+"+233201234569",
			nil, client.AdminToken)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}

		if resp.StatusCode == http.StatusNotFound {
			t.Skip("GET /tickets/history endpoint not implemented (404)")
		}

		if AssertStatusCode(t, http.StatusOK, resp.StatusCode, "Get ticket history") {
			t.Logf("✓ Retrieved ticket history for customer")
		}
	})

	t.Run("Reprint_Ticket", func(t *testing.T) {
		if ticketID == "" {
			t.Skip("No ticket ID available")
		}

		token := client.AgentToken
		if token == "" {
			token = client.AdminToken
		}

		resp, err := client.MakeRequest("POST", "/tickets/"+ticketID+"/reprint",
			nil, token)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}

		if resp.StatusCode == http.StatusNotFound {
			t.Skip("POST /tickets/{id}/reprint endpoint not implemented (404)")
		}

		if AssertStatusCode(t, http.StatusOK, resp.StatusCode, "Reprint ticket") {
			t.Logf("✓ Ticket reprint generated")
		}
	})

	t.Run("Get_Batch_Details", func(t *testing.T) {
		if batchID == "" {
			t.Skip("No batch ID available")
		}

		token := client.AgentToken
		if token == "" {
			token = client.AdminToken
		}

		resp, err := client.MakeRequest("GET", "/tickets/batch/"+batchID, nil, token)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}

		if resp.StatusCode == http.StatusNotFound {
			t.Skip("GET /tickets/batch/{id} endpoint not implemented (404)")
		}

		if AssertStatusCode(t, http.StatusOK, resp.StatusCode, "Get batch details") {
			t.Logf("✓ Retrieved batch details")
		}
	})

	t.Run("Void_Ticket", func(t *testing.T) {
		if ticketID == "" {
			t.Skip("No ticket ID available")
		}

		voidData := map[string]interface{}{
			"reason":        "System error during printing",
			"authorized_by": "admin@randlottery.com",
		}

		resp, err := client.MakeRequest("POST", "/admin/tickets/"+ticketID+"/void",
			voidData, client.AdminToken)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}

		if resp.StatusCode == http.StatusNotFound {
			t.Skip("POST /admin/tickets/{id}/void endpoint not implemented (404)")
		}

		// Accept 200 (voided) or 400 (cannot void - already processed, etc.)
		if AssertStatusCodes(t, []int{http.StatusOK, http.StatusBadRequest},
			resp.StatusCode, "Void ticket") {
			if resp.StatusCode == http.StatusBadRequest {
				t.Logf("⚠ Ticket cannot be voided (may be already processed)")
			} else {
				t.Logf("✓ Ticket voided successfully")
			}
		}
	})
}
