package tests

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"
)

const (
	// Test configuration
	APIGatewayURL = "http://localhost:4000"
	APIBasePath   = "/api/v1"

	// Seeded admin credentials
	SeededAdminEmail    = "admin@randlottery.com"
	SeededAdminPassword = "Admin@123"
)

// Test data - will use timestamp suffix to avoid conflicts
var (
	TestSuffix        = fmt.Sprintf("%d", time.Now().Unix())
	TestAdminEmail    = fmt.Sprintf("test.admin.%s@randlottery.com", TestSuffix)
	TestAdminUsername = fmt.Sprintf("testadmin%s", TestSuffix)
	TestAdminPassword = "TestAdmin@123"

	TestAgentCode     = fmt.Sprintf("AGT%s", TestSuffix)
	TestAgentEmail    = fmt.Sprintf("agent.%s@test.com", TestSuffix)
	TestAgentPassword = "Agent@123"
	TestDeviceID      = fmt.Sprintf("device-%s", TestSuffix)

	TestRetailerCode = fmt.Sprintf("RET%s", TestSuffix)
	TestRetailerName = fmt.Sprintf("Test Retailer %s", TestSuffix)
)

// Response structures
type APIResponse struct {
	Success   bool            `json:"success"`
	Message   string          `json:"message"`
	Data      json.RawMessage `json:"data,omitempty"`
	Error     *ErrorResponse  `json:"error,omitempty"`
	Meta      *MetaInfo       `json:"meta,omitempty"`
	Timestamp string          `json:"timestamp,omitempty"`
}

type ErrorResponse struct {
	Code    string          `json:"code"`
	Message string          `json:"message"`
	Details json.RawMessage `json:"details,omitempty"`
}

type MetaInfo struct {
	RequestID string `json:"request_id"`
	Version   string `json:"version"`
}

type LoginResponse struct {
	AccessToken  string          `json:"access_token"`
	RefreshToken string          `json:"refresh_token"`
	User         json.RawMessage `json:"user"`
}

type PaginatedResponse struct {
	Success    bool            `json:"success"`
	Message    string          `json:"message"`
	Data       json.RawMessage `json:"data"`
	Pagination PaginationInfo  `json:"pagination"`
}

type PaginationInfo struct {
	Page       int `json:"page"`
	PageSize   int `json:"page_size"`
	TotalCount int `json:"total_count"`
	TotalPages int `json:"total_pages"`
}

// TestClient wraps HTTP client for testing
type TestClient struct {
	HTTPClient *http.Client
	BaseURL    string
	AdminToken string
	AgentToken string
}

// NewTestClient creates a new test client
func NewTestClient() *TestClient {
	return &TestClient{
		HTTPClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		BaseURL: APIGatewayURL + APIBasePath,
	}
}

// MakeRequest makes an HTTP request to the API
func (tc *TestClient) MakeRequest(method, endpoint string, body interface{}, token string) (*http.Response, error) {
	var reqBody io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reqBody = bytes.NewBuffer(jsonBody)
	}

	// Build full URL
	url := tc.BaseURL + endpoint
	if endpoint[0] != '/' {
		url = tc.BaseURL + "/" + endpoint
	}

	req, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	return tc.HTTPClient.Do(req)
}

// ParseResponse parses HTTP response into target struct
func (tc *TestClient) ParseResponse(resp *http.Response, target interface{}) error {
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	return json.Unmarshal(body, target)
}

// CheckHealth checks if API Gateway is healthy
func (tc *TestClient) CheckHealth(t *testing.T) bool {
	resp, err := tc.HTTPClient.Get(APIGatewayURL + "/health")
	if err != nil {
		t.Logf("Health check failed: %v", err)
		return false
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Logf("Health check returned status: %d", resp.StatusCode)
		return false
	}

	return true
}

// LoginAsAdmin logs in with admin credentials and stores token
func (tc *TestClient) LoginAsAdmin(t *testing.T) bool {
	loginData := map[string]string{
		"email":    SeededAdminEmail,
		"password": SeededAdminPassword,
	}

	resp, err := tc.MakeRequest("POST", "/admin/auth/login", loginData, "")
	if err != nil {
		t.Logf("Admin login request failed: %v", err)
		return false
	}

	if resp.StatusCode != http.StatusOK {
		t.Logf("Admin login returned status: %d", resp.StatusCode)
		return false
	}

	var apiResp APIResponse
	if err := tc.ParseResponse(resp, &apiResp); err != nil {
		t.Logf("Failed to parse login response: %v", err)
		return false
	}

	if !apiResp.Success {
		t.Logf("Login response indicates failure: %s", apiResp.Message)
		return false
	}

	var loginResp LoginResponse
	if err := json.Unmarshal(apiResp.Data, &loginResp); err != nil {
		t.Logf("Failed to parse login data: %v", err)
		return false
	}

	tc.AdminToken = loginResp.AccessToken
	t.Logf("Admin login successful, token obtained")
	return true
}

// AssertStatusCode checks if response has expected status code
func AssertStatusCode(t *testing.T, expected, actual int, message string) bool {
	if expected != actual {
		t.Errorf("%s: expected status %d, got %d", message, expected, actual)
		return false
	}
	return true
}

// AssertStatusCodes checks if response has one of expected status codes
func AssertStatusCodes(t *testing.T, expected []int, actual int, message string) bool {
	for _, exp := range expected {
		if exp == actual {
			return true
		}
	}
	t.Errorf("%s: expected one of %v, got %d", message, expected, actual)
	return false
}
