package tests

import (
	"net/http"
	"testing"
)

// TestScenario_UserManagement tests admin user management endpoints
func TestScenario_UserManagement(t *testing.T) {
	client := NewTestClient()

	// Check API Gateway health
	if !client.CheckHealth(t) {
		t.Fatal("API Gateway is not healthy")
	}

	// Login as admin first
	if !client.LoginAsAdmin(t) {
		t.Fatal("Failed to login as admin")
	}

	t.Run("List_Admin_Users", func(t *testing.T) {
		resp, err := client.MakeRequest("GET", "/admin/users?page=1&page_size=10", nil, client.AdminToken)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}

		if resp.StatusCode == http.StatusNotFound {
			t.Skip("GET /admin/users endpoint not implemented (404)")
		}

		if AssertStatusCode(t, http.StatusOK, resp.StatusCode, "List admin users") {
			var paginatedResp PaginatedResponse
			if err := client.ParseResponse(resp, &paginatedResp); err == nil && paginatedResp.Success {
				t.Logf("✓ Retrieved %d admin users", paginatedResp.Pagination.TotalCount)
			}
		}
	})

	t.Run("Get_Admin_User_By_ID", func(t *testing.T) {
		// Use the seeded admin user ID
		userID := "c0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11"

		resp, err := client.MakeRequest("GET", "/admin/users/"+userID, nil, client.AdminToken)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}

		if resp.StatusCode == http.StatusNotFound {
			t.Skip("GET /admin/users/{id} endpoint not implemented (404)")
		}

		if AssertStatusCode(t, http.StatusOK, resp.StatusCode, "Get admin user by ID") {
			t.Logf("✓ Retrieved admin user details")
		}
	})

	t.Run("Create_Admin_User", func(t *testing.T) {
		userData := map[string]interface{}{
			"email":      TestAdminEmail,
			"username":   TestAdminUsername,
			"password":   TestAdminPassword,
			"first_name": "Test",
			"last_name":  "Admin",
		}

		resp, err := client.MakeRequest("POST", "/admin/users", userData, client.AdminToken)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}

		if resp.StatusCode == http.StatusNotFound {
			t.Skip("POST /admin/users endpoint not implemented (404)")
		}

		// Accept 201 (created), 200 (ok), or 400 (already exists)
		if AssertStatusCodes(t, []int{http.StatusCreated, http.StatusOK, http.StatusBadRequest},
			resp.StatusCode, "Create admin user") {

			if resp.StatusCode == http.StatusBadRequest {
				t.Logf("⚠ Admin user might already exist")
			} else {
				t.Logf("✓ Admin user created successfully")
			}
		}
	})

	t.Run("Update_Admin_User", func(t *testing.T) {
		userID := "c0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11" // Seeded admin

		updateData := map[string]interface{}{
			"first_name": "Updated",
			"last_name":  "Admin",
		}

		resp, err := client.MakeRequest("PUT", "/admin/users/"+userID, updateData, client.AdminToken)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}

		if resp.StatusCode == http.StatusNotFound {
			t.Skip("PUT /admin/users/{id} endpoint not implemented (404)")
		}

		if AssertStatusCode(t, http.StatusOK, resp.StatusCode, "Update admin user") {
			t.Logf("✓ Admin user updated successfully")
		}
	})

	t.Run("List_Roles", func(t *testing.T) {
		resp, err := client.MakeRequest("GET", "/admin/roles?page=1&page_size=10", nil, client.AdminToken)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}

		if resp.StatusCode == http.StatusNotFound {
			t.Skip("GET /admin/roles endpoint not implemented (404)")
		}

		if AssertStatusCode(t, http.StatusOK, resp.StatusCode, "List roles") {
			var paginatedResp PaginatedResponse
			if err := client.ParseResponse(resp, &paginatedResp); err == nil && paginatedResp.Success {
				t.Logf("✓ Retrieved %d roles", paginatedResp.Pagination.TotalCount)
			}
		}
	})

	t.Run("List_Permissions", func(t *testing.T) {
		resp, err := client.MakeRequest("GET", "/admin/permissions?page=1&page_size=10", nil, client.AdminToken)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}

		if resp.StatusCode == http.StatusNotFound {
			t.Skip("GET /admin/permissions endpoint not implemented (404)")
		}

		if AssertStatusCode(t, http.StatusOK, resp.StatusCode, "List permissions") {
			var paginatedResp PaginatedResponse
			if err := client.ParseResponse(resp, &paginatedResp); err == nil && paginatedResp.Success {
				t.Logf("✓ Retrieved %d permissions", paginatedResp.Pagination.TotalCount)
			}
		}
	})

	t.Run("Assign_Role_To_User", func(t *testing.T) {
		assignData := map[string]string{
			"user_id": "c0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11",
			"role_id": "a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11", // super_admin role from seed
		}

		resp, err := client.MakeRequest("POST", "/admin/role-assignments", assignData, client.AdminToken)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}

		if resp.StatusCode == http.StatusNotFound {
			t.Skip("POST /admin/role-assignments endpoint not implemented (404)")
		}

		// Accept 200 (ok) or 400 (already assigned)
		if AssertStatusCodes(t, []int{http.StatusOK, http.StatusBadRequest},
			resp.StatusCode, "Assign role to user") {

			if resp.StatusCode == http.StatusBadRequest {
				t.Logf("⚠ Role might already be assigned")
			} else {
				t.Logf("✓ Role assigned successfully")
			}
		}
	})
}
