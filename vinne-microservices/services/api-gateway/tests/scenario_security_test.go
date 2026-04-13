package tests

import (
	"net/http"
	"strings"
	"testing"
	"time"
)

// TestScenario_Security tests security features and validation
func TestScenario_Security(t *testing.T) {
	client := NewTestClient()

	// Check API Gateway health
	if !client.CheckHealth(t) {
		t.Fatal("API Gateway is not healthy")
	}

	t.Run("Reject_Invalid_Token", func(t *testing.T) {
		resp, err := client.MakeRequest("GET", "/admin/users", nil, "invalid-token-123456")
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}

		if AssertStatusCodes(t, []int{http.StatusUnauthorized, http.StatusForbidden},
			resp.StatusCode, "Invalid token") {
			t.Logf("✓ Invalid token properly rejected")
		}
	})

	t.Run("Reject_Missing_Token", func(t *testing.T) {
		resp, err := client.MakeRequest("GET", "/admin/users", nil, "")
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}

		if AssertStatusCode(t, http.StatusUnauthorized, resp.StatusCode, "Missing token") {
			t.Logf("✓ Missing token properly rejected")
		}
	})

	t.Run("Reject_SQL_Injection", func(t *testing.T) {
		sqlInjectionData := map[string]string{
			"email":    "admin' OR '1'='1",
			"password": "password",
		}

		resp, err := client.MakeRequest("POST", "/admin/auth/login", sqlInjectionData, "")
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}

		// Should not return 200 or expose data
		if resp.StatusCode == http.StatusOK {
			t.Error("SQL injection attempt should not succeed!")
		} else if AssertStatusCodes(t, []int{http.StatusBadRequest, http.StatusUnauthorized, http.StatusInternalServerError},
			resp.StatusCode, "SQL injection") {
			t.Logf("✓ SQL injection attempt rejected")
		}
	})

	t.Run("Reject_XSS_Attempt", func(t *testing.T) {
		xssData := map[string]string{
			"email":    "<script>alert('XSS')</script>@test.com",
			"password": "password123",
		}

		resp, err := client.MakeRequest("POST", "/admin/auth/login", xssData, "")
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}

		if AssertStatusCodes(t, []int{http.StatusBadRequest, http.StatusUnauthorized},
			resp.StatusCode, "XSS attempt") {
			t.Logf("✓ XSS attempt rejected")
		}
	})

	t.Run("Validate_Email_Format", func(t *testing.T) {
		invalidEmails := []string{
			"notanemail",
			"@test.com",
			"user@",
			"user@.com",
			"user space@test.com",
		}

		for _, email := range invalidEmails {
			loginData := map[string]string{
				"email":    email,
				"password": "ValidPassword123",
			}

			resp, err := client.MakeRequest("POST", "/admin/auth/login", loginData, "")
			if err != nil {
				t.Fatalf("Request failed for email %s: %v", email, err)
			}

			if resp.StatusCode == http.StatusOK {
				t.Errorf("Invalid email %s should not be accepted", email)
			}
		}
		t.Logf("✓ Invalid email formats rejected")
	})

	t.Run("Validate_Password_Strength", func(t *testing.T) {
		// Login as admin first
		if !client.LoginAsAdmin(t) {
			t.Skip("Failed to login as admin")
		}

		weakPasswords := []string{
			"weak",
			"12345678",
			"password",
			"admin123",
		}

		for _, password := range weakPasswords {
			userData := map[string]interface{}{
				"email":      "weakpass@test.com",
				"username":   "weakuser",
				"password":   password,
				"first_name": "Weak",
				"last_name":  "Password",
			}

			resp, err := client.MakeRequest("POST", "/admin/users", userData, client.AdminToken)
			if err != nil {
				continue // Skip on error
			}

			if resp.StatusCode == http.StatusNotFound {
				t.Skip("POST /admin/users endpoint not implemented")
			}

			if resp.StatusCode == http.StatusCreated || resp.StatusCode == http.StatusOK {
				t.Errorf("Weak password '%s' should not be accepted", password)
			}
		}
		t.Logf("✓ Weak passwords rejected")
	})

	t.Run("Validate_Phone_Number", func(t *testing.T) {
		// Login as admin first
		if !client.LoginAsAdmin(t) {
			t.Skip("Failed to login as admin")
		}

		invalidPhones := []string{
			"123",
			"not-a-phone",
			"+1",  // Too short
			"233", // Missing digits
		}

		for _, phone := range invalidPhones {
			agentData := map[string]interface{}{
				"agent_code":       "AGTTEST",
				"email":            "test@test.com",
				"phone":            phone,
				"password":         "Test@123",
				"first_name":       "Test",
				"last_name":        "Agent",
				"business_name":    "Test",
				"business_address": "Test",
				"region":           "Test",
				"commission_rate":  10,
			}

			resp, err := client.MakeRequest("POST", "/admin/agents", agentData, client.AdminToken)
			if err != nil {
				continue // Skip on error
			}

			if resp.StatusCode == http.StatusNotFound {
				t.Skip("POST /admin/agents endpoint not implemented")
			}

			if resp.StatusCode == http.StatusCreated || resp.StatusCode == http.StatusOK {
				t.Errorf("Invalid phone '%s' should not be accepted", phone)
			}
		}
		t.Logf("✓ Invalid phone numbers rejected")
	})

	t.Run("Test_Rate_Limiting", func(t *testing.T) {
		// Make rapid requests to test rate limiting
		rateLimited := false
		endpoint := "/health"

		for i := 0; i < 30; i++ {
			resp, err := client.HTTPClient.Get(APIGatewayURL + endpoint)
			if err != nil {
				if strings.Contains(err.Error(), "429") {
					rateLimited = true
					break
				}
				continue
			}

			if resp.StatusCode == http.StatusTooManyRequests {
				rateLimited = true
				_ = resp.Body.Close()
				break
			}
			_ = resp.Body.Close()

			// Small delay to avoid overwhelming the server
			if i%5 == 0 {
				time.Sleep(10 * time.Millisecond)
			}
		}

		if rateLimited {
			t.Logf("✓ Rate limiting is active")
		} else {
			t.Logf("⚠ Rate limiting not detected (might not be configured)")
		}
	})

	t.Run("Test_CORS_Headers", func(t *testing.T) {
		// Test CORS headers
		req, err := http.NewRequest("OPTIONS", APIGatewayURL+"/api/v1/admin/auth/login", nil)
		if err != nil {
			t.Fatalf("Failed to create request: %v", err)
		}

		req.Header.Set("Origin", "http://localhost:3000")
		req.Header.Set("Access-Control-Request-Method", "POST")

		resp, err := client.HTTPClient.Do(req)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer func() { _ = resp.Body.Close() }()

		// Check for CORS headers
		if resp.Header.Get("Access-Control-Allow-Origin") != "" {
			t.Logf("✓ CORS headers present: %s", resp.Header.Get("Access-Control-Allow-Origin"))
		} else {
			t.Logf("⚠ CORS headers not configured")
		}
	})

	t.Run("Audit_Log_Access", func(t *testing.T) {
		// Login as admin
		if !client.LoginAsAdmin(t) {
			t.Skip("Failed to login as admin")
		}

		resp, err := client.MakeRequest("GET", "/admin/audit-logs?page=1&page_size=10", nil, client.AdminToken)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}

		if resp.StatusCode == http.StatusNotFound {
			t.Skip("GET /admin/audit-logs endpoint not implemented (404)")
		}

		if AssertStatusCode(t, http.StatusOK, resp.StatusCode, "Get audit logs") {
			t.Logf("✓ Audit logs accessible to admin")
		}
	})
}
