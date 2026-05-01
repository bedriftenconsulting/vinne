package handlers

import (
	"net/http"
)

// AgentHandler defines the interface for agent-related operations
type AgentHandler interface {
	CreateAgent(w http.ResponseWriter, r *http.Request) error
	GetAgent(w http.ResponseWriter, r *http.Request) error
	GetAgentByCode(w http.ResponseWriter, r *http.Request) error
	ListAgents(w http.ResponseWriter, r *http.Request) error
	UpdateAgent(w http.ResponseWriter, r *http.Request) error
	UpdateAgentStatus(w http.ResponseWriter, r *http.Request) error
	GetAgentCommissions(w http.ResponseWriter, r *http.Request) error
	GetAgentOverview(w http.ResponseWriter, r *http.Request) error
}

// RetailerHandler defines the interface for retailer-related operations
type RetailerHandler interface {
	// Admin methods
	CreateRetailer(w http.ResponseWriter, r *http.Request) error
	GetRetailer(w http.ResponseWriter, r *http.Request) error
	ListRetailers(w http.ResponseWriter, r *http.Request) error
	UpdateRetailer(w http.ResponseWriter, r *http.Request) error
	UpdateRetailerStatus(w http.ResponseWriter, r *http.Request) error

	// Agent-specific methods
	ListAgentRetailers(w http.ResponseWriter, r *http.Request) error
	OnboardRetailer(w http.ResponseWriter, r *http.Request) error
	GetRetailerDetails(w http.ResponseWriter, r *http.Request) error
	GetRetailerPOSDevices(w http.ResponseWriter, r *http.Request) error
	GetRetailerPerformance(w http.ResponseWriter, r *http.Request) error
	GetRetailerWinningTickets(w http.ResponseWriter, r *http.Request) error
	CreatePOSRequest(w http.ResponseWriter, r *http.Request) error
	ListPOSRequests(w http.ResponseWriter, r *http.Request) error
	GetPOSRequest(w http.ResponseWriter, r *http.Request) error

	// Terminal methods
	GetAssignedTerminal(w http.ResponseWriter, r *http.Request) error
	UpdateHeartbeat(w http.ResponseWriter, r *http.Request) error
	GetAssignedTerminalConfig(w http.ResponseWriter, r *http.Request) error
}

// AdminAuthHandler interface is defined in admin_auth.go

// AdminUserHandler defines the interface for admin user management operations
type AdminUserHandler interface {
	CreateAdminUser(w http.ResponseWriter, r *http.Request) error
	GetAdminUser(w http.ResponseWriter, r *http.Request) error
	UpdateAdminUser(w http.ResponseWriter, r *http.Request) error
	DeleteAdminUser(w http.ResponseWriter, r *http.Request) error
	ListAdminUsers(w http.ResponseWriter, r *http.Request) error
	UpdateAdminUserStatus(w http.ResponseWriter, r *http.Request) error
}

// AdminRoleHandler defines the interface for admin role management operations
type AdminRoleHandler interface {
	CreateRole(w http.ResponseWriter, r *http.Request) error
	GetRole(w http.ResponseWriter, r *http.Request) error
	UpdateRole(w http.ResponseWriter, r *http.Request) error
	DeleteRole(w http.ResponseWriter, r *http.Request) error
	ListRoles(w http.ResponseWriter, r *http.Request) error
	AssignRole(w http.ResponseWriter, r *http.Request) error
	ListPermissions(w http.ResponseWriter, r *http.Request) error
}

// AdminAuditHandler defines the interface for admin audit operations
type AdminAuditHandler interface {
	GetAuditLogs(w http.ResponseWriter, r *http.Request) error
	GetUserActivity(w http.ResponseWriter, r *http.Request) error
	GetSystemEvents(w http.ResponseWriter, r *http.Request) error
}

// TerminalCRUDHandler defines basic terminal CRUD operations
type TerminalCRUDHandler interface {
	RegisterTerminal(w http.ResponseWriter, r *http.Request) error
	GetTerminal(w http.ResponseWriter, r *http.Request) error
	ListTerminals(w http.ResponseWriter, r *http.Request) error
	UpdateTerminal(w http.ResponseWriter, r *http.Request) error
	DeleteTerminal(w http.ResponseWriter, r *http.Request) error
}

// TerminalAssignmentHandler defines terminal assignment operations
type TerminalAssignmentHandler interface {
	AssignTerminal(w http.ResponseWriter, r *http.Request) error
	UnassignTerminal(w http.ResponseWriter, r *http.Request) error
}

// TerminalManagementHandler defines terminal management operations
type TerminalManagementHandler interface {
	GetTerminalHealth(w http.ResponseWriter, r *http.Request) error
	UpdateTerminalStatus(w http.ResponseWriter, r *http.Request) error
	GetTerminalConfig(w http.ResponseWriter, r *http.Request) error
	UpdateTerminalConfig(w http.ResponseWriter, r *http.Request) error
}

// TerminalHandler combines all terminal-related handlers
type TerminalHandler interface {
	TerminalCRUDHandler
	TerminalAssignmentHandler
	TerminalManagementHandler
}

// GameCRUDHandler defines basic game CRUD operations
type GameCRUDHandler interface {
	CreateGame(w http.ResponseWriter, r *http.Request) error
	GetGame(w http.ResponseWriter, r *http.Request) error
	ListGames(w http.ResponseWriter, r *http.Request) error
	UpdateGame(w http.ResponseWriter, r *http.Request) error
	DeleteGame(w http.ResponseWriter, r *http.Request) error
	CloneGame(w http.ResponseWriter, r *http.Request) error
}

// GameApprovalHandler defines game approval workflow operations
type GameApprovalHandler interface {
	SubmitForApproval(w http.ResponseWriter, r *http.Request) error
	ApproveGame(w http.ResponseWriter, r *http.Request) error
	RejectGame(w http.ResponseWriter, r *http.Request) error
	GetApprovalStatus(w http.ResponseWriter, r *http.Request) error
	GetPendingApprovals(w http.ResponseWriter, r *http.Request) error
}

// GameConfigHandler defines game configuration operations
type GameConfigHandler interface {
	GetPrizeStructure(w http.ResponseWriter, r *http.Request) error
	UpdatePrizeStructure(w http.ResponseWriter, r *http.Request) error
	GetGameRules(w http.ResponseWriter, r *http.Request) error
	UpdateGameRules(w http.ResponseWriter, r *http.Request) error
}

// GameScheduleHandler defines game scheduling operations
type GameScheduleHandler interface {
	ScheduleGame(w http.ResponseWriter, r *http.Request) error
	GetGameSchedule(w http.ResponseWriter, r *http.Request) error
	UpdateScheduledGame(w http.ResponseWriter, r *http.Request) error
	UpdateGameStatus(w http.ResponseWriter, r *http.Request) error
	GetGameStatistics(w http.ResponseWriter, r *http.Request) error
}

// GameHandler combines all game-related handlers
type GameHandler interface {
	GameCRUDHandler
	GameApprovalHandler
	GameConfigHandler
	GameScheduleHandler
}

// DrawCRUDHandler defines basic draw CRUD operations
type DrawCRUDHandler interface {
	CreateDraw(w http.ResponseWriter, r *http.Request) error
	GetDraw(w http.ResponseWriter, r *http.Request) error
	ListDraws(w http.ResponseWriter, r *http.Request) error
	UpdateDraw(w http.ResponseWriter, r *http.Request) error
	DeleteDraw(w http.ResponseWriter, r *http.Request) error
	BulkUploadTickets(w http.ResponseWriter, r *http.Request) error
}

// DrawExecutionHandler defines draw execution operations
type DrawExecutionHandler interface {
	PrepareDraw(w http.ResponseWriter, r *http.Request) error
	ExecuteDraw(w http.ResponseWriter, r *http.Request) error
	SaveDrawProgress(w http.ResponseWriter, r *http.Request) error
	RestartDraw(w http.ResponseWriter, r *http.Request) error
	RecordPhysicalDraw(w http.ResponseWriter, r *http.Request) error
}

// DrawResultHandler defines draw result operations
type DrawResultHandler interface {
	CommitDrawResults(w http.ResponseWriter, r *http.Request) error
	GetDrawResults(w http.ResponseWriter, r *http.Request) error
	GetWinningNumbers(w http.ResponseWriter, r *http.Request) error
	ValidateDraw(w http.ResponseWriter, r *http.Request) error
	GetDrawStatistics(w http.ResponseWriter, r *http.Request) error
	GetDrawTickets(w http.ResponseWriter, r *http.Request) error
	GetPublicCompletedDraws(w http.ResponseWriter, r *http.Request) error
	GetPublicWinners(w http.ResponseWriter, r *http.Request) error
	GetCompletedDraws(w http.ResponseWriter, r *http.Request) error
	UpdateMachineNumbers(w http.ResponseWriter, r *http.Request) error
	GetAgentDrawHistory(w http.ResponseWriter, r *http.Request) error
}

// DrawPayoutHandler defines draw payout operations
type DrawPayoutHandler interface {
	ProcessPayout(w http.ResponseWriter, r *http.Request) error
}

// DrawHandler combines all draw-related handlers
type DrawHandler interface {
	DrawCRUDHandler
	DrawExecutionHandler
	DrawResultHandler
	DrawPayoutHandler
}
