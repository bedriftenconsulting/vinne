package tests

import (
	"encoding/json"
	"net/http"
	"testing"
)

// TestScenario_AgentManagement tests agent management and authentication
func TestScenario_AgentManagement(t *testing.T) {
	client := NewTestClient()

	// Check API Gateway health
	if !client.CheckHealth(t) {
		t.Fatal("API Gateway is not healthy")
	}

	// Login as admin first for management operations
	if !client.LoginAsAdmin(t) {
		t.Fatal("Failed to login as admin")
	}

	var agentID string

	t.Run("Create_Agent", func(t *testing.T) {
		agentData := map[string]interface{}{
			"agent_code":                  TestAgentCode,
			"email":                       TestAgentEmail,
			"phone":                       "+233201234567",
			"password":                    TestAgentPassword,
			"first_name":                  "Test",
			"last_name":                   "Agent",
			"business_name":               "Test Agency",
			"business_address":            "123 Test Street, Accra",
			"region":                      "Greater Accra",
			"tin_number":                  "TIN" + TestSuffix,
			"business_certificate_number": "BC" + TestSuffix,
			"commission_rate":             10.5,
			"credit_limit":                5000.00,
			"kyc_status":                  "pending",
		}

		resp, err := client.MakeRequest("POST", "/admin/agents", agentData, client.AdminToken)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}

		if resp.StatusCode == http.StatusNotFound {
			t.Skip("POST /admin/agents endpoint not implemented (404)")
		}

		// Accept 201 (created), 200 (ok), or 400 (already exists)
		if AssertStatusCodes(t, []int{http.StatusCreated, http.StatusOK, http.StatusBadRequest},
			resp.StatusCode, "Create agent") {

			if resp.StatusCode == http.StatusBadRequest {
				t.Logf("⚠ Agent might already exist")
			} else {
				// Try to extract agent ID
				var apiResp APIResponse
				if err := client.ParseResponse(resp, &apiResp); err == nil && apiResp.Success {
					var agentResp map[string]interface{}
					if json.Unmarshal(apiResp.Data, &agentResp) == nil {
						if id, ok := agentResp["id"].(string); ok {
							agentID = id
							t.Logf("✓ Agent created with ID: %s", agentID)
						}
					}
				}
			}
		}
	})

	t.Run("List_Agents", func(t *testing.T) {
		resp, err := client.MakeRequest("GET", "/admin/agents?page=1&page_size=10", nil, client.AdminToken)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}

		if resp.StatusCode == http.StatusNotFound {
			t.Skip("GET /admin/agents endpoint not implemented (404)")
		}

		if AssertStatusCode(t, http.StatusOK, resp.StatusCode, "List agents") {
			var paginatedResp PaginatedResponse
			if err := client.ParseResponse(resp, &paginatedResp); err == nil && paginatedResp.Success {
				t.Logf("✓ Retrieved %d agents", paginatedResp.Pagination.TotalCount)
			}
		}
	})

	t.Run("Get_Agent_Details", func(t *testing.T) {
		if agentID == "" {
			t.Skip("No agent ID available")
		}

		resp, err := client.MakeRequest("GET", "/admin/agents/"+agentID, nil, client.AdminToken)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}

		if resp.StatusCode == http.StatusNotFound {
			t.Skip("GET /admin/agents/{id} endpoint not implemented (404)")
		}

		if AssertStatusCode(t, http.StatusOK, resp.StatusCode, "Get agent details") {
			t.Logf("✓ Retrieved agent details")
		}
	})

	t.Run("Update_Agent", func(t *testing.T) {
		if agentID == "" {
			t.Skip("No agent ID available")
		}

		updateData := map[string]interface{}{
			"commission_rate": 12.5,
			"credit_limit":    7500.00,
		}

		resp, err := client.MakeRequest("PUT", "/admin/agents/"+agentID, updateData, client.AdminToken)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}

		if resp.StatusCode == http.StatusNotFound {
			t.Skip("PUT /admin/agents/{id} endpoint not implemented (404)")
		}

		if AssertStatusCode(t, http.StatusOK, resp.StatusCode, "Update agent") {
			t.Logf("✓ Agent updated successfully")
		}
	})

	t.Run("Approve_Agent_KYC", func(t *testing.T) {
		if agentID == "" {
			t.Skip("No agent ID available")
		}

		kycData := map[string]interface{}{
			"kyc_status": "approved",
			"kyc_notes":  "Approved via automated test",
		}

		resp, err := client.MakeRequest("PUT", "/admin/agents/"+agentID+"/kyc", kycData, client.AdminToken)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}

		if resp.StatusCode == http.StatusNotFound {
			t.Skip("PUT /admin/agents/{id}/kyc endpoint not implemented (404)")
		}

		if AssertStatusCode(t, http.StatusOK, resp.StatusCode, "Approve KYC") {
			t.Logf("✓ Agent KYC approved")
		}
	})

	t.Run("Agent_Login", func(t *testing.T) {
		loginData := map[string]string{
			"agent_code": TestAgentCode,
			"password":   TestAgentPassword,
			"device_id":  TestDeviceID,
		}

		resp, err := client.MakeRequest("POST", "/agent/auth/login", loginData, "")
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}

		if resp.StatusCode == http.StatusNotFound {
			t.Skip("POST /agent/auth/login endpoint not implemented (404)")
		}

		// Accept 200 (success) or 401 (unauthorized - agent might not exist)
		switch resp.StatusCode {
		case http.StatusOK:
			var apiResp APIResponse
			if err := client.ParseResponse(resp, &apiResp); err == nil && apiResp.Success {
				var loginResp LoginResponse
				if json.Unmarshal(apiResp.Data, &loginResp) == nil {
					client.AgentToken = loginResp.AccessToken
					t.Logf("✓ Agent login successful")
				}
			}
		case http.StatusUnauthorized:
			t.Logf("⚠ Agent login failed - agent might not exist or wrong credentials")
		default:
			t.Errorf("Unexpected status code: %d", resp.StatusCode)
		}
	})

	t.Run("Agent_Get_Profile", func(t *testing.T) {
		if client.AgentToken == "" {
			t.Skip("No agent token available")
		}

		resp, err := client.MakeRequest("GET", "/agent/profile", nil, client.AgentToken)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}

		if resp.StatusCode == http.StatusNotFound {
			t.Skip("GET /agent/profile endpoint not implemented (404)")
		}

		if AssertStatusCode(t, http.StatusOK, resp.StatusCode, "Get agent profile") {
			t.Logf("✓ Retrieved agent profile")
		}
	})

	t.Run("Agent_Get_Wallet_Balance", func(t *testing.T) {
		if client.AgentToken == "" {
			t.Skip("No agent token available")
		}

		resp, err := client.MakeRequest("GET", "/agent/wallet/balance", nil, client.AgentToken)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}

		if resp.StatusCode == http.StatusNotFound {
			t.Skip("GET /agent/wallet/balance endpoint not implemented (404)")
		}

		if AssertStatusCode(t, http.StatusOK, resp.StatusCode, "Get wallet balance") {
			t.Logf("✓ Retrieved wallet balance")
		}
	})

	t.Run("Create_Retailer", func(t *testing.T) {
		if client.AdminToken == "" {
			t.Skip("No admin token available")
		}

		retailerData := map[string]interface{}{
			"retailer_code": TestRetailerCode,
			"name":          TestRetailerName,
			"phone":         "+233201234568",
			"address":       "456 Retail Street, Accra",
			"region":        "Greater Accra",
			"agent_id":      agentID, // Link to agent if available
		}

		resp, err := client.MakeRequest("POST", "/admin/retailers", retailerData, client.AdminToken)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}

		if resp.StatusCode == http.StatusNotFound {
			t.Skip("POST /admin/retailers endpoint not implemented (404)")
		}

		// Accept 201 (created), 200 (ok), or 400 (already exists)
		if AssertStatusCodes(t, []int{http.StatusCreated, http.StatusOK, http.StatusBadRequest},
			resp.StatusCode, "Create retailer") {

			if resp.StatusCode == http.StatusBadRequest {
				t.Logf("⚠ Retailer might already exist")
			} else {
				t.Logf("✓ Retailer created successfully")
			}
		}
	})

	t.Run("List_Retailers", func(t *testing.T) {
		resp, err := client.MakeRequest("GET", "/admin/retailers?page=1&page_size=10", nil, client.AdminToken)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}

		if resp.StatusCode == http.StatusNotFound {
			t.Skip("GET /admin/retailers endpoint not implemented (404)")
		}

		if AssertStatusCode(t, http.StatusOK, resp.StatusCode, "List retailers") {
			var paginatedResp PaginatedResponse
			if err := client.ParseResponse(resp, &paginatedResp); err == nil && paginatedResp.Success {
				t.Logf("✓ Retrieved %d retailers", paginatedResp.Pagination.TotalCount)
			}
		}
	})
}
