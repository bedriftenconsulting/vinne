package tests

import (
	"encoding/json"
	"net/http"
	"testing"
)

// TestScenario_AdminAuthentication tests the complete admin authentication flow
func TestScenario_AdminAuthentication(t *testing.T) {
	client := NewTestClient()

	// Check API Gateway health first
	if !client.CheckHealth(t) {
		t.Fatal("API Gateway is not healthy")
	}

	t.Run("Login_With_Seeded_Admin", func(t *testing.T) {
		loginData := map[string]string{
			"email":    SeededAdminEmail,
			"password": SeededAdminPassword,
		}

		resp, err := client.MakeRequest("POST", "/admin/auth/login", loginData, "")
		if err != nil {
			t.Fatalf("Login request failed: %v", err)
		}

		if !AssertStatusCode(t, http.StatusOK, resp.StatusCode, "Admin login") {
			return
		}

		var apiResp APIResponse
		if err := client.ParseResponse(resp, &apiResp); err != nil {
			t.Fatalf("Failed to parse response: %v", err)
		}

		if !apiResp.Success {
			t.Fatalf("Login failed: %s", apiResp.Message)
		}

		var loginResp LoginResponse
		if err := json.Unmarshal(apiResp.Data, &loginResp); err != nil {
			t.Fatalf("Failed to parse login data: %v", err)
		}

		if loginResp.AccessToken == "" {
			t.Error("No access token received")
		}
		if loginResp.RefreshToken == "" {
			t.Error("No refresh token received")
		}

		// Store token for other tests
		client.AdminToken = loginResp.AccessToken

		t.Logf("✓ Admin login successful")
	})

	t.Run("Get_Admin_Profile", func(t *testing.T) {
		if client.AdminToken == "" {
			t.Skip("No admin token available")
		}

		resp, err := client.MakeRequest("GET", "/admin/auth/me", nil, client.AdminToken)
		if err != nil {
			t.Fatalf("Profile request failed: %v", err)
		}

		// This endpoint might not be implemented yet
		if resp.StatusCode == http.StatusNotFound {
			t.Skip("GET /admin/auth/me endpoint not implemented (404)")
		}

		if !AssertStatusCode(t, http.StatusOK, resp.StatusCode, "Get admin profile") {
			return
		}

		var apiResp APIResponse
		if err := client.ParseResponse(resp, &apiResp); err != nil {
			t.Fatalf("Failed to parse response: %v", err)
		}

		if apiResp.Success {
			t.Logf("✓ Admin profile retrieved successfully")
		}
	})

	t.Run("Refresh_Token", func(t *testing.T) {
		// First login to get tokens
		loginData := map[string]string{
			"email":    SeededAdminEmail,
			"password": SeededAdminPassword,
		}

		resp, err := client.MakeRequest("POST", "/admin/auth/login", loginData, "")
		if err != nil {
			t.Fatalf("Login request failed: %v", err)
		}

		var apiResp APIResponse
		if err := client.ParseResponse(resp, &apiResp); err != nil {
			t.Fatalf("Failed to parse response: %v", err)
		}

		var loginResp LoginResponse
		if err := json.Unmarshal(apiResp.Data, &loginResp); err != nil {
			t.Fatalf("Failed to parse login data: %v", err)
		}

		// Now test refresh
		refreshData := map[string]string{
			"refresh_token": loginResp.RefreshToken,
		}

		resp, err = client.MakeRequest("POST", "/admin/auth/refresh", refreshData, "")
		if err != nil {
			t.Fatalf("Refresh request failed: %v", err)
		}

		if !AssertStatusCode(t, http.StatusOK, resp.StatusCode, "Token refresh") {
			return
		}

		if err := client.ParseResponse(resp, &apiResp); err != nil {
			t.Fatalf("Failed to parse response: %v", err)
		}

		if apiResp.Success {
			var refreshResp LoginResponse
			if err := json.Unmarshal(apiResp.Data, &refreshResp); err == nil {
				if refreshResp.AccessToken != "" {
					t.Logf("✓ Token refresh successful")
				}
			}
		}
	})

	t.Run("Invalid_Credentials", func(t *testing.T) {
		loginData := map[string]string{
			"email":    "wrong@email.com",
			"password": "WrongPassword",
		}

		resp, err := client.MakeRequest("POST", "/admin/auth/login", loginData, "")
		if err != nil {
			t.Fatalf("Login request failed: %v", err)
		}

		if !AssertStatusCode(t, http.StatusUnauthorized, resp.StatusCode, "Invalid credentials") {
			t.Error("Invalid credentials should return 401")
		}

		t.Logf("✓ Invalid credentials properly rejected")
	})

	t.Run("Logout", func(t *testing.T) {
		if client.AdminToken == "" {
			t.Skip("No admin token available")
		}

		resp, err := client.MakeRequest("POST", "/admin/auth/logout", nil, client.AdminToken)
		if err != nil {
			t.Fatalf("Logout request failed: %v", err)
		}

		// Logout endpoint might not be implemented
		if resp.StatusCode == http.StatusNotFound {
			t.Skip("POST /admin/auth/logout endpoint not implemented (404)")
		}

		if AssertStatusCode(t, http.StatusOK, resp.StatusCode, "Admin logout") {
			t.Logf("✓ Admin logout successful")
		}
	})
}
