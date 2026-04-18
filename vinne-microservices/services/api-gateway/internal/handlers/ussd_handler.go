package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/randco/randco-microservices/services/api-gateway/internal/grpc"
	"github.com/randco/randco-microservices/shared/common/logger"
)

// USSDHandler handles mNotify USSD callbacks
// mNotify sends POST to /api/v1/ussd/callback with:
//
//	{ "msisdn": "233541509394", "sequenceID": "6789940010", "data": "*899*86#", "timestamp": "..." }
//
// We respond with:
//
//	{ "msisdn": "...", "sequenceID": "...", "message": "...", "timestamp": "...", "continueFlag": 0 }
type USSDHandler struct {
	grpcManager *grpc.ClientManager
	logger      logger.Logger
}

// mNotify Shared USSD request payload
type ussdRequest struct {
	MSISDN     string `json:"msisdn"`
	SequenceID string `json:"sequenceID"`
	Data       string `json:"data"`
	Timestamp  string `json:"timestamp"`
}

// mNotify Shared USSD response payload
type ussdResponse struct {
	MSISDN       string `json:"msisdn"`
	SequenceID   string `json:"sequenceID"`
	Message      string `json:"message"`
	Timestamp    string `json:"timestamp"`
	ContinueFlag int    `json:"continueFlag"` // 0 = continue session, 1 = end session
}

// USSD menu states
const (
	ussdMenuMain     = "MAIN"
	ussdMenuRegister = "REGISTER"
	ussdMenuLogin    = "LOGIN"
	ussdMenuBalance  = "BALANCE"
	ussdMenuBuyTicket = "BUY_TICKET"
	ussdMenuResults  = "RESULTS"
)

// NewUSSDHandler creates a new USSD handler
func NewUSSDHandler(grpcManager *grpc.ClientManager, logger logger.Logger) *USSDHandler {
	return &USSDHandler{
		grpcManager: grpcManager,
		logger:      logger,
	}
}

// HandleCallback handles the mNotify USSD shared callback
// POST /api/v1/ussd/callback
func (h *USSDHandler) HandleCallback(w http.ResponseWriter, r *http.Request) error {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	_ = ctx

	// Parse request
	var req ussdRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Error("Failed to decode USSD request", "error", err)
		return h.respond(w, ussdResponse{
			Message:      "Service error. Please try again.",
			ContinueFlag: 1,
		})
	}

	h.logger.Info("USSD callback received",
		"msisdn", req.MSISDN,
		"sequence_id", req.SequenceID,
		"data", req.Data,
	)

	// Normalise phone number (strip leading zeros, ensure 233 prefix)
	phone := normalisePhone(req.MSISDN)

	// Route based on user input
	input := strings.TrimSpace(req.Data)
	resp := h.routeUSSD(phone, req.SequenceID, input)
	resp.MSISDN = req.MSISDN
	resp.SequenceID = req.SequenceID
	resp.Timestamp = time.Now().UTC().Format(time.RFC3339)

	return h.respond(w, resp)
}

// routeUSSD processes the USSD input and returns the appropriate menu
func (h *USSDHandler) routeUSSD(phone, sequenceID, input string) ussdResponse {
	// Initial dial — show main menu
	// mNotify sends the shortcode as data on first request e.g. "*899*86#"
	if strings.Contains(input, "#") || input == "" {
		return ussdResponse{
			Message:      "Welcome to WinBig Africa\r\n1. Buy Ticket\r\n2. Check Results\r\n3. My Account\r\n4. Register\r\n0. Exit",
			ContinueFlag: 0,
		}
	}

	switch input {
	case "1":
		return ussdResponse{
			Message:      "Buy Ticket\r\nEnter game code to buy ticket\r\nor 0 to go back:",
			ContinueFlag: 0,
		}
	case "2":
		return ussdResponse{
			Message:      "Latest Results\r\nVisit winbig.bedriften.xyz\r\nfor full results\r\n0. Back",
			ContinueFlag: 0,
		}
	case "3":
		return ussdResponse{
			Message:      "My Account\r\n1. Check Balance\r\n2. My Tickets\r\n3. Change PIN\r\n0. Back",
			ContinueFlag: 0,
		}
	case "4":
		return ussdResponse{
			Message:      "Register for WinBig\r\nEnter your first name:",
			ContinueFlag: 0,
		}
	case "0":
		return ussdResponse{
			Message:      "Thank you for using WinBig Africa. Good luck!",
			ContinueFlag: 1,
		}
	default:
		return ussdResponse{
			Message:      "Invalid option.\r\n1. Buy Ticket\r\n2. Check Results\r\n3. My Account\r\n4. Register\r\n0. Exit",
			ContinueFlag: 0,
		}
	}
}

// respond writes the USSD JSON response
func (h *USSDHandler) respond(w http.ResponseWriter, resp ussdResponse) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	return json.NewEncoder(w).Encode(resp)
}

// normalisePhone ensures phone is in 233XXXXXXXXX format
func normalisePhone(phone string) string {
	phone = strings.TrimSpace(phone)
	// Remove + prefix
	phone = strings.TrimPrefix(phone, "+")
	// Convert 0XXXXXXXXX to 233XXXXXXXXX
	if strings.HasPrefix(phone, "0") {
		phone = "233" + phone[1:]
	}
	return phone
}
