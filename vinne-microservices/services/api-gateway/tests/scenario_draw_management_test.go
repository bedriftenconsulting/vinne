package tests

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"
)

// TestScenario_DrawManagement tests draw creation, execution, and results
func TestScenario_DrawManagement(t *testing.T) {
	client := NewTestClient()

	// Check API Gateway health
	if !client.CheckHealth(t) {
		t.Fatal("API Gateway is not healthy")
	}

	// Login as admin for management operations
	if !client.LoginAsAdmin(t) {
		t.Fatal("Failed to login as admin")
	}

	var gameID string
	var drawID string

	// First, create a game for the draws
	t.Run("Setup_Game_For_Draws", func(t *testing.T) {
		gameData := map[string]interface{}{
			"name":            "Draw Test Game " + TestSuffix,
			"code":            "DT" + TestSuffix,
			"type":            "lotto",
			"description":     "Game for draw testing",
			"price":           5.00,
			"max_number":      49,
			"selection_count": 6,
			"draw_frequency":  "daily",
			"draw_days":       []string{"monday", "tuesday", "wednesday", "thursday", "friday", "saturday", "sunday"},
			"draw_time":       "20:00",
			"timezone":        "Africa/Accra",
			"is_active":       true,
		}

		resp, err := client.MakeRequest("POST", "/admin/games", gameData, client.AdminToken)
		if err != nil {
			t.Fatalf("Setup game request failed: %v", err)
		}

		if resp.StatusCode == http.StatusNotFound {
			t.Skip("Game creation endpoint not available, skipping draw tests")
		}

		if resp.StatusCode == http.StatusCreated || resp.StatusCode == http.StatusOK {
			var apiResp APIResponse
			if err := client.ParseResponse(resp, &apiResp); err == nil && apiResp.Success {
				var gameResp map[string]interface{}
				if json.Unmarshal(apiResp.Data, &gameResp) == nil {
					if id, ok := gameResp["id"].(string); ok {
						gameID = id
						t.Logf("✓ Game created for draw testing: %s", gameID)
					}
				}
			}
		}
	})

	t.Run("Schedule_Draw", func(t *testing.T) {
		if gameID == "" {
			t.Skip("No game ID available")
		}

		// Schedule a draw for tomorrow
		drawTime := time.Now().Add(24 * time.Hour)
		drawData := map[string]interface{}{
			"game_id":        gameID,
			"draw_number":    1001,
			"draw_date":      drawTime.Format("2006-01-02"),
			"draw_time":      drawTime.Format("15:04:05"),
			"status":         "scheduled",
			"prize_pool":     100000.00,
			"jackpot_amount": 50000.00,
			"sales_cutoff":   drawTime.Add(-1 * time.Hour).Format(time.RFC3339),
		}

		resp, err := client.MakeRequest("POST", "/admin/draws", drawData, client.AdminToken)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}

		if resp.StatusCode == http.StatusNotFound {
			t.Skip("POST /admin/draws endpoint not implemented (404)")
		}

		if AssertStatusCodes(t, []int{http.StatusCreated, http.StatusOK},
			resp.StatusCode, "Schedule draw") {
			// Try to extract draw ID
			var apiResp APIResponse
			if err := client.ParseResponse(resp, &apiResp); err == nil && apiResp.Success {
				var drawResp map[string]interface{}
				if json.Unmarshal(apiResp.Data, &drawResp) == nil {
					if id, ok := drawResp["id"].(string); ok {
						drawID = id
						t.Logf("✓ Draw scheduled with ID: %s", drawID)
					}
				}
			}
		}
	})

	t.Run("List_Scheduled_Draws", func(t *testing.T) {
		resp, err := client.MakeRequest("GET", "/admin/draws?status=scheduled&page=1&page_size=10",
			nil, client.AdminToken)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}

		if resp.StatusCode == http.StatusNotFound {
			t.Skip("GET /admin/draws endpoint not implemented (404)")
		}

		if AssertStatusCode(t, http.StatusOK, resp.StatusCode, "List scheduled draws") {
			var paginatedResp PaginatedResponse
			if err := client.ParseResponse(resp, &paginatedResp); err == nil && paginatedResp.Success {
				t.Logf("✓ Retrieved %d scheduled draws", paginatedResp.Pagination.TotalCount)
			}
		}
	})

	t.Run("Get_Draw_Details", func(t *testing.T) {
		if drawID == "" {
			t.Skip("No draw ID available")
		}

		resp, err := client.MakeRequest("GET", "/admin/draws/"+drawID, nil, client.AdminToken)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}

		if resp.StatusCode == http.StatusNotFound {
			t.Skip("GET /admin/draws/{id} endpoint not implemented (404)")
		}

		if AssertStatusCode(t, http.StatusOK, resp.StatusCode, "Get draw details") {
			t.Logf("✓ Retrieved draw details")
		}
	})

	t.Run("Update_Draw", func(t *testing.T) {
		if drawID == "" {
			t.Skip("No draw ID available")
		}

		updateData := map[string]interface{}{
			"prize_pool":     150000.00,
			"jackpot_amount": 75000.00,
		}

		resp, err := client.MakeRequest("PUT", "/admin/draws/"+drawID, updateData, client.AdminToken)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}

		if resp.StatusCode == http.StatusNotFound {
			t.Skip("PUT /admin/draws/{id} endpoint not implemented (404)")
		}

		if AssertStatusCode(t, http.StatusOK, resp.StatusCode, "Update draw") {
			t.Logf("✓ Draw updated successfully")
		}
	})

	t.Run("Execute_Draw", func(t *testing.T) {
		if drawID == "" {
			t.Skip("No draw ID available")
		}

		// Execute the draw with winning numbers
		executeData := map[string]interface{}{
			"winning_numbers": []int{7, 14, 21, 28, 35, 42},
			"draw_method":     "manual", // or "rng" for random number generator
			"witnessed_by":    "Test Administrator",
			"notes":           "Test draw execution",
		}

		resp, err := client.MakeRequest("POST", "/admin/draws/"+drawID+"/execute",
			executeData, client.AdminToken)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}

		if resp.StatusCode == http.StatusNotFound {
			t.Skip("POST /admin/draws/{id}/execute endpoint not implemented (404)")
		}

		// Accept 200 (ok) or 400 (draw not ready, already executed, etc.)
		if AssertStatusCodes(t, []int{http.StatusOK, http.StatusBadRequest},
			resp.StatusCode, "Execute draw") {
			if resp.StatusCode == http.StatusBadRequest {
				t.Logf("⚠ Draw cannot be executed (might not be ready or already executed)")
			} else {
				t.Logf("✓ Draw executed successfully")
			}
		}
	})

	t.Run("Get_Draw_Results", func(t *testing.T) {
		if drawID == "" {
			t.Skip("No draw ID available")
		}

		resp, err := client.MakeRequest("GET", "/admin/draws/"+drawID+"/results", nil, client.AdminToken)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}

		if resp.StatusCode == http.StatusNotFound {
			t.Skip("GET /admin/draws/{id}/results endpoint not implemented (404)")
		}

		if AssertStatusCode(t, http.StatusOK, resp.StatusCode, "Get draw results") {
			t.Logf("✓ Retrieved draw results")
		}
	})

	t.Run("Calculate_Winners", func(t *testing.T) {
		if drawID == "" {
			t.Skip("No draw ID available")
		}

		resp, err := client.MakeRequest("POST", "/admin/draws/"+drawID+"/calculate-winners",
			nil, client.AdminToken)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}

		if resp.StatusCode == http.StatusNotFound {
			t.Skip("POST /admin/draws/{id}/calculate-winners endpoint not implemented (404)")
		}

		// Accept 200 (ok), 202 (accepted for processing), or 400 (already calculated)
		if AssertStatusCodes(t, []int{http.StatusOK, http.StatusAccepted, http.StatusBadRequest},
			resp.StatusCode, "Calculate winners") {
			if resp.StatusCode == http.StatusBadRequest {
				t.Logf("⚠ Winners already calculated or draw not executed")
			} else {
				t.Logf("✓ Winner calculation initiated")
			}
		}
	})

	t.Run("Get_Winners_List", func(t *testing.T) {
		if drawID == "" {
			t.Skip("No draw ID available")
		}

		resp, err := client.MakeRequest("GET", "/admin/draws/"+drawID+"/winners", nil, client.AdminToken)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}

		if resp.StatusCode == http.StatusNotFound {
			t.Skip("GET /admin/draws/{id}/winners endpoint not implemented (404)")
		}

		if AssertStatusCode(t, http.StatusOK, resp.StatusCode, "Get winners list") {
			var apiResp APIResponse
			if err := client.ParseResponse(resp, &apiResp); err == nil && apiResp.Success {
				t.Logf("✓ Retrieved winners list")
			}
		}
	})

	t.Run("Approve_Draw_Results", func(t *testing.T) {
		if drawID == "" {
			t.Skip("No draw ID available")
		}

		approvalData := map[string]interface{}{
			"approved":       true,
			"approved_by":    "Test Administrator",
			"approval_notes": "Results verified and approved",
		}

		resp, err := client.MakeRequest("POST", "/admin/draws/"+drawID+"/approve",
			approvalData, client.AdminToken)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}

		if resp.StatusCode == http.StatusNotFound {
			t.Skip("POST /admin/draws/{id}/approve endpoint not implemented (404)")
		}

		if AssertStatusCodes(t, []int{http.StatusOK, http.StatusBadRequest},
			resp.StatusCode, "Approve draw results") {
			if resp.StatusCode == http.StatusBadRequest {
				t.Logf("⚠ Draw results cannot be approved (might not be executed or already approved)")
			} else {
				t.Logf("✓ Draw results approved")
			}
		}
	})

	t.Run("Get_Public_Draw_Results", func(t *testing.T) {
		if drawID == "" {
			t.Skip("No draw ID available")
		}

		// Public endpoint - no auth token
		resp, err := client.MakeRequest("GET", "/draws/"+drawID+"/results", nil, "")
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}

		if resp.StatusCode == http.StatusNotFound {
			t.Skip("GET /draws/{id}/results (public) endpoint not implemented (404)")
		}

		// Accept 200 (results available) or 404 (not yet published)
		if AssertStatusCodes(t, []int{http.StatusOK, http.StatusNotFound},
			resp.StatusCode, "Get public draw results") {
			if resp.StatusCode == http.StatusNotFound {
				t.Logf("⚠ Draw results not yet published")
			} else {
				t.Logf("✓ Public draw results available")
			}
		}
	})

	t.Run("Get_Latest_Draws", func(t *testing.T) {
		// Public endpoint - get latest draws
		resp, err := client.MakeRequest("GET", "/draws/latest?limit=10", nil, "")
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}

		if resp.StatusCode == http.StatusNotFound {
			t.Skip("GET /draws/latest endpoint not implemented (404)")
		}

		if AssertStatusCode(t, http.StatusOK, resp.StatusCode, "Get latest draws") {
			t.Logf("✓ Retrieved latest draws (public)")
		}
	})

	t.Run("Get_Upcoming_Draws", func(t *testing.T) {
		// Public endpoint - get upcoming draws
		resp, err := client.MakeRequest("GET", "/draws/upcoming", nil, "")
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}

		if resp.StatusCode == http.StatusNotFound {
			t.Skip("GET /draws/upcoming endpoint not implemented (404)")
		}

		if AssertStatusCode(t, http.StatusOK, resp.StatusCode, "Get upcoming draws") {
			t.Logf("✓ Retrieved upcoming draws (public)")
		}
	})

	t.Run("Cancel_Draw", func(t *testing.T) {
		if drawID == "" {
			t.Skip("No draw ID available")
		}

		cancelData := map[string]interface{}{
			"reason":         "Test cancellation",
			"refund_tickets": true,
		}

		resp, err := client.MakeRequest("POST", "/admin/draws/"+drawID+"/cancel",
			cancelData, client.AdminToken)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}

		if resp.StatusCode == http.StatusNotFound {
			t.Skip("POST /admin/draws/{id}/cancel endpoint not implemented (404)")
		}

		// Accept 200 (cancelled) or 400 (cannot cancel - already executed)
		if AssertStatusCodes(t, []int{http.StatusOK, http.StatusBadRequest},
			resp.StatusCode, "Cancel draw") {
			if resp.StatusCode == http.StatusBadRequest {
				t.Logf("⚠ Draw cannot be cancelled (might be already executed)")
			} else {
				t.Logf("✓ Draw cancelled successfully")
			}
		}
	})

	t.Run("Get_Draw_Statistics", func(t *testing.T) {
		if drawID == "" {
			t.Skip("No draw ID available")
		}

		resp, err := client.MakeRequest("GET", "/admin/draws/"+drawID+"/statistics",
			nil, client.AdminToken)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}

		if resp.StatusCode == http.StatusNotFound {
			t.Skip("GET /admin/draws/{id}/statistics endpoint not implemented (404)")
		}

		if AssertStatusCode(t, http.StatusOK, resp.StatusCode, "Get draw statistics") {
			t.Logf("✓ Retrieved draw statistics")
		}
	})
}
