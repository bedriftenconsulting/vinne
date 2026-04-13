package handlers

import (
	"context"
	"net/http"
	"time"

	agentmgmtpb "github.com/randco/randco-microservices/proto/agent/management/v1"
	ticketpb "github.com/randco/randco-microservices/proto/ticket/v1"
	"github.com/randco/randco-microservices/services/api-gateway/internal/config"
	"github.com/randco/randco-microservices/services/api-gateway/internal/grpc"
	"github.com/randco/randco-microservices/services/api-gateway/internal/router"
	"github.com/randco/randco-microservices/shared/common/logger"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// agentHandlerImpl handles agent-related requests
type agentHandlerImpl struct {
	grpcManager *grpc.ClientManager
	log         logger.Logger
	config      *config.Config
}

// NewAgentHandler creates a new agent handler
func NewAgentHandler(grpcManager *grpc.ClientManager, log logger.Logger, cfg *config.Config) AgentHandler {
	return &agentHandlerImpl{
		grpcManager: grpcManager,
		log:         log,
		config:      cfg,
	}
}

// Helper method for converting agent proto to map
func convertAgentToMap(agent *agentmgmtpb.Agent) map[string]interface{} {
	if agent == nil {
		return nil
	}

	result := map[string]interface{}{
		"id":                    agent.Id,
		"agent_code":            agent.AgentCode,
		"name":                  agent.Name,
		"email":                 agent.Email,
		"phone_number":          agent.PhoneNumber,
		"address":               agent.Address,
		"commission_percentage": agent.CommissionPercentage,
		"status":                convertStatusEnumToString(agent.Status),
		"created_by":            agent.CreatedBy,
		"created_at":            agent.CreatedAt.AsTime().Format("2006-01-02T15:04:05Z"),
		"updated_at":            agent.UpdatedAt.AsTime().Format("2006-01-02T15:04:05Z"),
	}

	if agent.InitialPassword != "" {
		result["initial_password"] = agent.InitialPassword
	}

	return result
}

// Helper method for converting retailer proto to map
// TODO: Uncomment when retailer service is implemented
// func convertRetailerToMap(retailer *agentmgmtpb.Retailer) map[string]interface{} {
// 	if retailer == nil {
// 		return nil
// 	}

// 	result := map[string]interface{}{
// 		"id":            retailer.Id,
// 		"retailer_code": retailer.RetailerCode,
// 		"name":          retailer.Name,
// 		"email":         retailer.Email,
// 		"phone_number":  retailer.PhoneNumber,
// 		"address":       retailer.Address,
// 		"agent_id":      retailer.AgentId,
// 		"status":        convertStatusEnumToString(retailer.Status),
// 		"created_by":    retailer.CreatedBy,
// 		"created_at":    retailer.CreatedAt.AsTime().Format("2006-01-02T15:04:05Z"),
// 		"updated_at":    retailer.UpdatedAt.AsTime().Format("2006-01-02T15:04:05Z"),
// 	}

// 	return result
// }

func (h *agentHandlerImpl) GetAgentOverview(w http.ResponseWriter, r *http.Request) error {
	agentID := router.GetUserID(r)
	if agentID == "" {
		return router.ErrorResponse(w, http.StatusBadRequest, "agent_id is required")
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	amClient, err := h.grpcManager.AgentManagementClient()
	if err != nil {
		h.log.Error("Failed to get agent management client", "error", err)
		return router.ErrorResponse(w, http.StatusServiceUnavailable, "Service unavailable")
	}

	retailersResp, err := amClient.GetAgentRetailers(ctx, &agentmgmtpb.GetAgentRetailersRequest{AgentId: agentID})
	if err != nil {
		h.log.Error("GetAgentRetailers failed", "error", err)
		return router.ErrorResponse(w, http.StatusBadGateway, "Failed to fetch retailers")
	}
	totalRetailers := 0
	if retailersResp != nil && len(retailersResp.Retailers) > 0 {
		totalRetailers = len(retailersResp.Retailers)
	}

	totalTicketsSold := int64(0)
	tClient, err := h.grpcManager.TicketServiceClient()
	if err == nil && retailersResp != nil {
		// Define a default time window (last 30 days) to scope counts
		end := time.Now().UTC()
		start := end.AddDate(0, 0, -30)
		for _, rtr := range retailersResp.Retailers {
			// Some systems use retailer_code as issuer_id
			filter := &ticketpb.TicketFilter{
				IssuerType: "retailer",
				IssuerId:   rtr.RetailerCode,
				StartDate:  timestamppb.New(start),
				EndDate:    timestamppb.New(end),
			}
			req := &ticketpb.ListTicketsRequest{Filter: filter, Page: 1, PageSize: 1}
			resp, err := tClient.ListTickets(ctx, req)
			if err != nil {
				// Log and continue to next retailer
				h.log.Error("ListTickets failed for retailer", "retailer_code", rtr.RetailerCode, "error", err)
				continue
			}
			totalTicketsSold += resp.GetTotal()
		}
	} else if err != nil {
		h.log.Warn("Ticket service client unavailable; tickets_sold set to 0", "error", err)
	}

	// Commission earned: not yet aggregated here (requires wallet summary RPC)
	totalCommissionEarned := int64(0)

	return router.WriteJSON(w, http.StatusOK, map[string]any{
		"totalRetailers":        totalRetailers,
		"totalTicketsSold":      totalTicketsSold,
		"totalCommissionEarned": totalCommissionEarned,
	})
}
