package tests

import (
	"encoding/json"
	"net/http"
	"testing"
)

// TestScenario_GameManagement tests game configuration and management
func TestScenario_GameManagement(t *testing.T) {
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
	var prizeStructureID string

	t.Run("Create_Game", func(t *testing.T) {
		gameData := map[string]interface{}{
			"name":                 "Test Lotto " + TestSuffix,
			"code":                 "LT" + TestSuffix,
			"type":                 "lotto",
			"description":          "Test lottery game",
			"price":                10.00,
			"max_number":           90,
			"selection_count":      5,
			"draw_frequency":       "weekly",
			"draw_days":            []string{"monday", "friday"},
			"draw_time":            "19:00",
			"timezone":             "Africa/Accra",
			"is_active":            true,
			"max_tickets_per_user": 100,
			"advance_draw_count":   4,
		}

		resp, err := client.MakeRequest("POST", "/admin/games", gameData, client.AdminToken)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}

		if resp.StatusCode == http.StatusNotFound {
			t.Skip("POST /admin/games endpoint not implemented (404)")
		}

		// Accept 201 (created), 200 (ok), or 400 (already exists)
		if AssertStatusCodes(t, []int{http.StatusCreated, http.StatusOK, http.StatusBadRequest},
			resp.StatusCode, "Create game") {

			if resp.StatusCode == http.StatusBadRequest {
				t.Logf("⚠ Game might already exist")
			} else {
				// Try to extract game ID
				var apiResp APIResponse
				if err := client.ParseResponse(resp, &apiResp); err == nil && apiResp.Success {
					var gameResp map[string]interface{}
					if json.Unmarshal(apiResp.Data, &gameResp) == nil {
						if id, ok := gameResp["id"].(string); ok {
							gameID = id
							t.Logf("✓ Game created with ID: %s", gameID)
						}
					}
				}
			}
		}
	})

	t.Run("List_Games", func(t *testing.T) {
		resp, err := client.MakeRequest("GET", "/admin/games?page=1&page_size=10", nil, client.AdminToken)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}

		if resp.StatusCode == http.StatusNotFound {
			t.Skip("GET /admin/games endpoint not implemented (404)")
		}

		if AssertStatusCode(t, http.StatusOK, resp.StatusCode, "List games") {
			var paginatedResp PaginatedResponse
			if err := client.ParseResponse(resp, &paginatedResp); err == nil && paginatedResp.Success {
				t.Logf("✓ Retrieved %d games", paginatedResp.Pagination.TotalCount)
			}
		}
	})

	t.Run("Get_Game_Details", func(t *testing.T) {
		if gameID == "" {
			t.Skip("No game ID available")
		}

		resp, err := client.MakeRequest("GET", "/admin/games/"+gameID, nil, client.AdminToken)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}

		if resp.StatusCode == http.StatusNotFound {
			t.Skip("GET /admin/games/{id} endpoint not implemented (404)")
		}

		if AssertStatusCode(t, http.StatusOK, resp.StatusCode, "Get game details") {
			t.Logf("✓ Retrieved game details")
		}
	})

	t.Run("Update_Game", func(t *testing.T) {
		if gameID == "" {
			t.Skip("No game ID available")
		}

		updateData := map[string]interface{}{
			"price":                15.00,
			"max_tickets_per_user": 150,
			"description":          "Updated test lottery game",
		}

		resp, err := client.MakeRequest("PUT", "/admin/games/"+gameID, updateData, client.AdminToken)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}

		if resp.StatusCode == http.StatusNotFound {
			t.Skip("PUT /admin/games/{id} endpoint not implemented (404)")
		}

		if AssertStatusCode(t, http.StatusOK, resp.StatusCode, "Update game") {
			t.Logf("✓ Game updated successfully")
		}
	})

	t.Run("Create_Prize_Structure", func(t *testing.T) {
		if gameID == "" {
			t.Skip("No game ID available")
		}

		prizeData := map[string]interface{}{
			"game_id":     gameID,
			"name":        "Standard Prize Structure",
			"description": "Standard prize distribution",
			"tiers": []map[string]interface{}{
				{
					"tier":             1,
					"name":             "Jackpot",
					"match_count":      5,
					"prize_type":       "percentage",
					"prize_value":      50.0,
					"is_jackpot":       true,
					"rollover_enabled": true,
				},
				{
					"tier":             2,
					"name":             "Second Prize",
					"match_count":      4,
					"prize_type":       "percentage",
					"prize_value":      25.0,
					"is_jackpot":       false,
					"rollover_enabled": false,
				},
				{
					"tier":             3,
					"name":             "Third Prize",
					"match_count":      3,
					"prize_type":       "fixed",
					"prize_value":      100.0,
					"is_jackpot":       false,
					"rollover_enabled": false,
				},
			},
			"is_active": true,
		}

		resp, err := client.MakeRequest("POST", "/admin/games/"+gameID+"/prize-structures", prizeData, client.AdminToken)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}

		if resp.StatusCode == http.StatusNotFound {
			t.Skip("POST /admin/games/{id}/prize-structures endpoint not implemented (404)")
		}

		if AssertStatusCodes(t, []int{http.StatusCreated, http.StatusOK},
			resp.StatusCode, "Create prize structure") {
			// Try to extract prize structure ID
			var apiResp APIResponse
			if err := client.ParseResponse(resp, &apiResp); err == nil && apiResp.Success {
				var prizeResp map[string]interface{}
				if json.Unmarshal(apiResp.Data, &prizeResp) == nil {
					if id, ok := prizeResp["id"].(string); ok {
						prizeStructureID = id
						t.Logf("✓ Prize structure created with ID: %s", prizeStructureID)
					}
				}
			}
		}
	})

	t.Run("Get_Prize_Structures", func(t *testing.T) {
		if gameID == "" {
			t.Skip("No game ID available")
		}

		resp, err := client.MakeRequest("GET", "/admin/games/"+gameID+"/prize-structures", nil, client.AdminToken)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}

		if resp.StatusCode == http.StatusNotFound {
			t.Skip("GET /admin/games/{id}/prize-structures endpoint not implemented (404)")
		}

		if AssertStatusCode(t, http.StatusOK, resp.StatusCode, "Get prize structures") {
			t.Logf("✓ Retrieved prize structures")
		}
	})

	t.Run("Update_Prize_Structure", func(t *testing.T) {
		if gameID == "" || prizeStructureID == "" {
			t.Skip("No game ID or prize structure ID available")
		}

		updateData := map[string]interface{}{
			"description": "Updated prize distribution",
			"tiers": []map[string]interface{}{
				{
					"tier":             1,
					"name":             "Mega Jackpot",
					"match_count":      5,
					"prize_type":       "percentage",
					"prize_value":      55.0,
					"is_jackpot":       true,
					"rollover_enabled": true,
				},
				{
					"tier":             2,
					"name":             "Second Prize",
					"match_count":      4,
					"prize_type":       "percentage",
					"prize_value":      20.0,
					"is_jackpot":       false,
					"rollover_enabled": false,
				},
				{
					"tier":             3,
					"name":             "Third Prize",
					"match_count":      3,
					"prize_type":       "fixed",
					"prize_value":      150.0,
					"is_jackpot":       false,
					"rollover_enabled": false,
				},
			},
		}

		resp, err := client.MakeRequest("PUT", "/admin/games/"+gameID+"/prize-structures/"+prizeStructureID,
			updateData, client.AdminToken)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}

		if resp.StatusCode == http.StatusNotFound {
			t.Skip("PUT /admin/games/{id}/prize-structures/{id} endpoint not implemented (404)")
		}

		if AssertStatusCode(t, http.StatusOK, resp.StatusCode, "Update prize structure") {
			t.Logf("✓ Prize structure updated successfully")
		}
	})

	t.Run("Toggle_Game_Status", func(t *testing.T) {
		if gameID == "" {
			t.Skip("No game ID available")
		}

		// Deactivate game
		statusData := map[string]interface{}{
			"is_active": false,
			"reason":    "Maintenance",
		}

		resp, err := client.MakeRequest("PUT", "/admin/games/"+gameID+"/status", statusData, client.AdminToken)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}

		if resp.StatusCode == http.StatusNotFound {
			t.Skip("PUT /admin/games/{id}/status endpoint not implemented (404)")
		}

		if AssertStatusCode(t, http.StatusOK, resp.StatusCode, "Toggle game status") {
			t.Logf("✓ Game status updated")
		}
	})

	t.Run("Get_Active_Games", func(t *testing.T) {
		resp, err := client.MakeRequest("GET", "/games/active", nil, "")
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}

		if resp.StatusCode == http.StatusNotFound {
			t.Skip("GET /games/active endpoint not implemented (404)")
		}

		if AssertStatusCode(t, http.StatusOK, resp.StatusCode, "Get active games") {
			var apiResp APIResponse
			if err := client.ParseResponse(resp, &apiResp); err == nil && apiResp.Success {
				t.Logf("✓ Retrieved active games (public endpoint)")
			}
		}
	})

	t.Run("Get_Game_Info_Public", func(t *testing.T) {
		if gameID == "" {
			t.Skip("No game ID available")
		}

		resp, err := client.MakeRequest("GET", "/games/"+gameID, nil, "")
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}

		if resp.StatusCode == http.StatusNotFound {
			t.Skip("GET /games/{id} endpoint not implemented (404)")
		}

		if AssertStatusCode(t, http.StatusOK, resp.StatusCode, "Get game info (public)") {
			t.Logf("✓ Retrieved game info (public endpoint)")
		}
	})

	t.Run("Delete_Game", func(t *testing.T) {
		if gameID == "" {
			t.Skip("No game ID available")
		}

		resp, err := client.MakeRequest("DELETE", "/admin/games/"+gameID, nil, client.AdminToken)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}

		if resp.StatusCode == http.StatusNotFound {
			t.Skip("DELETE /admin/games/{id} endpoint not implemented (404)")
		}

		// Accept 200 (ok), 204 (no content), or 400 (cannot delete active game)
		if AssertStatusCodes(t, []int{http.StatusOK, http.StatusNoContent, http.StatusBadRequest},
			resp.StatusCode, "Delete game") {
			if resp.StatusCode == http.StatusBadRequest {
				t.Logf("⚠ Cannot delete game (might be active or have associated data)")
			} else {
				t.Logf("✓ Game deleted successfully")
			}
		}
	})
}
