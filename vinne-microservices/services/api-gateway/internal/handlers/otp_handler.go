package handlers

import (
	"bytes"
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	jwtpkg "github.com/randco/randco-microservices/shared/common/jwt"
	grpcpkg "github.com/randco/randco-microservices/services/api-gateway/internal/grpc"
	"github.com/randco/randco-microservices/services/api-gateway/internal/router"
	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/bcrypt"

	_ "github.com/lib/pq"
)

const (
	mnotifyAPIKey = "F9XhjQbbJnqKt2fy9lhPIQCSD"
	mnotifySender = "CARPARK"
	mnotifyURL    = "https://api.mnotify.com/api/sms/quick"
	otpTTL        = 5 * time.Minute
	verifiedTTL   = 30 * 24 * time.Hour
)

type OTPHandler struct {
	redis       *redis.Client
	jwtService  jwtpkg.Service
	grpcManager *grpcpkg.ClientManager
}

func NewOTPHandler(redisClient *redis.Client, jwtService jwtpkg.Service, grpcManager *grpcpkg.ClientManager) *OTPHandler {
	return &OTPHandler{redis: redisClient, jwtService: jwtService, grpcManager: grpcManager}
}

type otpSendRequest struct {
	PlayerID string `json:"player_id"`
	Channel  string `json:"channel"`
	Contact  string `json:"contact"`
}

type otpVerifyRequest struct {
	PlayerID string `json:"player_id"`
	Channel  string `json:"channel"`
	Code     string `json:"code"`
}

// Send generates and sends an OTP to the given contact (phone only for now).
func (h *OTPHandler) Send(w http.ResponseWriter, r *http.Request) error {
	var req otpSendRequest
	if err := router.ReadJSON(r, &req); err != nil {
		return router.ErrorResponse(w, http.StatusBadRequest, "Invalid request body")
	}
	if req.PlayerID == "" || req.Channel == "" || req.Contact == "" {
		return router.ErrorResponse(w, http.StatusBadRequest, "player_id, channel and contact are required")
	}
	if req.Channel != "phone" {
		return router.ErrorResponse(w, http.StatusBadRequest, "Only phone channel is supported")
	}

	code, err := generate6DigitOTP()
	if err != nil {
		return router.ErrorResponse(w, http.StatusInternalServerError, "Failed to generate OTP")
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	key := fmt.Sprintf("phone_otp:%s", req.PlayerID)
	if err := h.redis.Set(ctx, key, code, otpTTL).Err(); err != nil {
		return router.ErrorResponse(w, http.StatusInternalServerError, "Failed to store OTP")
	}

	phone := normalisePhoneForOTP(req.Contact)
	msg := fmt.Sprintf("Your WinBig Africa verification code is: %s. Valid for 5 minutes.", code)
	if err := sendMNotifySMS(phone, msg); err != nil {
		return router.ErrorResponse(w, http.StatusInternalServerError, "Failed to send SMS: "+err.Error())
	}

	return router.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "OTP sent successfully",
	})
}

// Verify checks the submitted OTP code against what is stored in Redis.
func (h *OTPHandler) Verify(w http.ResponseWriter, r *http.Request) error {
	var req otpVerifyRequest
	if err := router.ReadJSON(r, &req); err != nil {
		return router.ErrorResponse(w, http.StatusBadRequest, "Invalid request body")
	}
	if req.PlayerID == "" || req.Channel == "" || req.Code == "" {
		return router.ErrorResponse(w, http.StatusBadRequest, "player_id, channel and code are required")
	}
	if req.Channel != "phone" {
		return router.ErrorResponse(w, http.StatusBadRequest, "Only phone channel is supported")
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	key := fmt.Sprintf("phone_otp:%s", req.PlayerID)
	stored, err := h.redis.Get(ctx, key).Result()
	if err == redis.Nil {
		return router.ErrorResponse(w, http.StatusBadRequest, "OTP expired or not found. Please request a new one.")
	}
	if err != nil {
		return router.ErrorResponse(w, http.StatusInternalServerError, "Verification failed")
	}
	if stored != strings.TrimSpace(req.Code) {
		return router.ErrorResponse(w, http.StatusBadRequest, "Invalid OTP code")
	}

	h.redis.Del(ctx, key)
	h.redis.Set(ctx, fmt.Sprintf("phone_verified:%s", req.PlayerID), "1", verifiedTTL)

	return router.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Phone number verified successfully",
	})
}

// Status returns phone/email verification flags for a player.
func (h *OTPHandler) Status(w http.ResponseWriter, r *http.Request) error {
	playerID := router.GetParam(r, "player_id")
	if playerID == "" {
		return router.ErrorResponse(w, http.StatusBadRequest, "player_id is required")
	}

	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	phoneVerified := h.redis.Exists(ctx, fmt.Sprintf("phone_verified:%s", playerID)).Val() > 0
	emailVerified := h.redis.Exists(ctx, fmt.Sprintf("email_verified:%s", playerID)).Val() > 0

	return router.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"phone_verified": phoneVerified,
		"email_verified": emailVerified,
	})
}

// USSDLoginRequest is the request body for the USSD OTP login check.
type USSDLoginRequest struct {
	PhoneNumber string `json:"phone_number"`
}

// USSDLoginVerifyRequest is the request body for verifying the USSD OTP.
type USSDLoginVerifyRequest struct {
	PhoneNumber string `json:"phone_number"`
	Code        string `json:"code"`
}

// USSDCheckAndSend checks if a USSD player exists by phone, then sends an OTP.
func (h *OTPHandler) USSDCheckAndSend(w http.ResponseWriter, r *http.Request) error {
	var req USSDLoginRequest
	if err := router.ReadJSON(r, &req); err != nil {
		return router.ErrorResponse(w, http.StatusBadRequest, "Invalid request body")
	}
	if req.PhoneNumber == "" {
		return router.ErrorResponse(w, http.StatusBadRequest, "phone_number is required")
	}

	// Normalise to +233 format for DB lookup
	dbPhone := toE164(req.PhoneNumber)

	playerID, err := lookupUSSDPlayer(dbPhone)
	if err != nil || playerID == "" {
		return router.ErrorResponse(w, http.StatusNotFound, "No tickets found for this number. Please check the number or sign up.")
	}

	code, err := generate6DigitOTP()
	if err != nil {
		return router.ErrorResponse(w, http.StatusInternalServerError, "Failed to generate OTP")
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	// Store "code:playerID" keyed by normalised phone, TTL 5 min
	redisKey := fmt.Sprintf("ussd_login_otp:%s", dbPhone)
	if err := h.redis.Set(ctx, redisKey, code+":"+playerID, otpTTL).Err(); err != nil {
		return router.ErrorResponse(w, http.StatusInternalServerError, "Failed to store OTP")
	}

	smsPhone := normalisePhoneForOTP(req.PhoneNumber)
	msg := fmt.Sprintf("Your WinBig Africa login code is: %s. Valid for 5 minutes. Do not share this code.", code)
	if err := sendMNotifySMS(smsPhone, msg); err != nil {
		return router.ErrorResponse(w, http.StatusInternalServerError, "Failed to send SMS: "+err.Error())
	}

	return router.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "OTP sent to your phone",
	})
}

// USSDVerifyAndLogin verifies the OTP and issues a JWT for the USSD player.
func (h *OTPHandler) USSDVerifyAndLogin(w http.ResponseWriter, r *http.Request) error {
	var req USSDLoginVerifyRequest
	if err := router.ReadJSON(r, &req); err != nil {
		return router.ErrorResponse(w, http.StatusBadRequest, "Invalid request body")
	}
	if req.PhoneNumber == "" || req.Code == "" {
		return router.ErrorResponse(w, http.StatusBadRequest, "phone_number and code are required")
	}

	dbPhone := toE164(req.PhoneNumber)
	redisKey := fmt.Sprintf("ussd_login_otp:%s", dbPhone)

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	stored, err := h.redis.Get(ctx, redisKey).Result()
	if err == redis.Nil {
		return router.ErrorResponse(w, http.StatusBadRequest, "OTP expired or not found. Please request a new one.")
	}
	if err != nil {
		return router.ErrorResponse(w, http.StatusInternalServerError, "Verification failed")
	}

	parts := strings.SplitN(stored, ":", 2)
	if len(parts) != 2 || parts[0] != strings.TrimSpace(req.Code) {
		return router.ErrorResponse(w, http.StatusBadRequest, "Invalid OTP code")
	}
	playerID := parts[1]

	h.redis.Del(ctx, redisKey)

	token, err := h.jwtService.GenerateAccessToken(jwtpkg.Claims{
		UserID:  playerID,
		Phone:   dbPhone,
		Channel: "ussd",
		JTI:     uuid.New().String(),
	})
	if err != nil {
		return router.ErrorResponse(w, http.StatusInternalServerError, "Failed to issue token")
	}

	return router.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"success":      true,
		"access_token": token,
		"profile": map[string]interface{}{
			"id":           playerID,
			"phone_number": dbPhone,
		},
	})
}

// lookupUSSDPlayer returns the player UUID only if the phone has at least one
// completed ticket in the ticket_service DB. This ensures the flow is restricted
// to actual ticket buyers.
func lookupUSSDPlayer(e164Phone string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Ticket DB stores phones as 233XXXXXXXXX (no leading +)
	ticketPhone := strings.TrimPrefix(e164Phone, "+")

	ticketDSN := os.Getenv("TICKET_DB_DSN")
	if ticketDSN == "" {
		ticketDSN = "host=service-ticket-db port=5432 user=ticket password=#kettic@333! dbname=ticket_service sslmode=disable"
	}
	tdb, err := sql.Open("postgres", ticketDSN)
	if err != nil {
		return "", err
	}
	defer tdb.Close()

	var count int
	err = tdb.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM tickets WHERE customer_phone = $1 AND payment_status = 'completed'`,
		ticketPhone,
	).Scan(&count)
	if err != nil || count == 0 {
		return "", nil
	}

	// Phone has tickets — get the player UUID from player_service
	playerDSN := os.Getenv("PLAYER_DB_DSN")
	if playerDSN == "" {
		playerDSN = "host=service-player-db port=5432 user=player password=#yerpla@333! dbname=player_service sslmode=disable"
	}
	pdb, err := sql.Open("postgres", playerDSN)
	if err != nil {
		return "", err
	}
	defer pdb.Close()

	var playerID string
	err = pdb.QueryRowContext(ctx,
		`SELECT id FROM players WHERE phone_number = $1 AND status = 'ACTIVE' LIMIT 1`,
		e164Phone,
	).Scan(&playerID)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return playerID, err
}

// lookupPlayerByPhone returns the player UUID for any registered player with the given E.164 phone.
func lookupPlayerByPhone(e164Phone string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	playerDSN := os.Getenv("PLAYER_DB_DSN")
	if playerDSN == "" {
		playerDSN = "host=service-player-db port=5432 user=player password=#yerpla@333! dbname=player_service sslmode=disable"
	}
	pdb, err := sql.Open("postgres", playerDSN)
	if err != nil {
		return "", err
	}
	defer pdb.Close()

	var playerID string
	err = pdb.QueryRowContext(ctx,
		`SELECT id FROM players WHERE phone_number = $1 AND status = 'ACTIVE' LIMIT 1`,
		e164Phone,
	).Scan(&playerID)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return playerID, err
}

// toE164 converts 0XXXXXXXXX or 233XXXXXXXXX to +233XXXXXXXXX.
func toE164(phone string) string {
	phone = strings.TrimSpace(phone)
	if strings.HasPrefix(phone, "+233") {
		return phone
	}
	if strings.HasPrefix(phone, "233") {
		return "+" + phone
	}
	if strings.HasPrefix(phone, "0") {
		return "+233" + phone[1:]
	}
	return phone
}

func generate6DigitOTP() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(1000000))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%06d", n.Int64()), nil
}

// normalisePhoneForOTP converts +233XXXXXXXXX or 233XXXXXXXXX to 0XXXXXXXXX for mNotify.
func normalisePhoneForOTP(phone string) string {
	phone = strings.TrimSpace(phone)
	if strings.HasPrefix(phone, "+233") {
		return "0" + phone[4:]
	}
	if strings.HasPrefix(phone, "233") {
		return "0" + phone[3:]
	}
	return phone
}

func sendMNotifySMS(phone, message string) error {
	payload, _ := json.Marshal(map[string]interface{}{
		"recipient":     []string{phone},
		"sender":        mnotifySender,
		"message":       message,
		"is_schedule":   false,
		"schedule_date": "",
	})

	url := fmt.Sprintf("%s?key=%s", mnotifyURL, mnotifyAPIKey)
	resp, err := http.Post(url, "application/json", bytes.NewReader(payload))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	if status, _ := result["status"].(string); status != "success" {
		return fmt.Errorf("mNotify error: %v", result["message"])
	}
	return nil
}

// ---------------------------------------------------------------------------
// Forgot / Reset Password
// ---------------------------------------------------------------------------

type forgotPasswordRequest struct {
	PhoneNumber string `json:"phone_number"`
}

type resetPasswordRequest struct {
	PhoneNumber string `json:"phone_number"`
	Code        string `json:"code"`
	NewPassword string `json:"new_password"`
}

// ForgotPassword sends an OTP to the player's phone for password reset.
// Uses the player service to verify the phone exists, then sends OTP via mNotify.
func (h *OTPHandler) ForgotPassword(w http.ResponseWriter, r *http.Request) error {
	var req forgotPasswordRequest
	if err := router.ReadJSON(r, &req); err != nil {
		return router.ErrorResponse(w, http.StatusBadRequest, "Invalid request body")
	}
	if req.PhoneNumber == "" {
		return router.ErrorResponse(w, http.StatusBadRequest, "phone_number is required")
	}

	phone := toE164(req.PhoneNumber)

	successMsg := map[string]interface{}{
		"success": true,
		"message": "If this number is registered, you'll receive a reset code shortly.",
	}

	// Look up player directly in the player DB — avoids gRPC dependency
	playerID, err := lookupPlayerByPhone(phone)
	if err != nil {
		fmt.Printf("[ForgotPassword] DB lookup error for %s: %v\n", phone, err)
		return router.WriteJSON(w, http.StatusOK, successMsg)
	}
	if playerID == "" {
		fmt.Printf("[ForgotPassword] No player found for %s\n", phone)
		return router.WriteJSON(w, http.StatusOK, successMsg)
	}
	fmt.Printf("[ForgotPassword] Found player %s for %s\n", playerID, phone)

	code, err := generate6DigitOTP()
	if err != nil {
		return router.ErrorResponse(w, http.StatusInternalServerError, "Failed to generate OTP")
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	redisKey := fmt.Sprintf("pwd_reset_otp:%s", phone)
	if err := h.redis.Set(ctx, redisKey, code+":"+playerID, otpTTL).Err(); err != nil {
		return router.ErrorResponse(w, http.StatusInternalServerError, "Failed to store OTP")
	}

	smsPhone := normalisePhoneForOTP(req.PhoneNumber)
	msg := fmt.Sprintf("Your WinBig Africa password reset code is: %s. Valid for 5 minutes. Do not share.", code)
	if err := sendMNotifySMS(smsPhone, msg); err != nil {
		return router.ErrorResponse(w, http.StatusInternalServerError, "Failed to send SMS: "+err.Error())
	}

	return router.WriteJSON(w, http.StatusOK, successMsg)
}

// ResetPassword verifies the OTP (stored in shared Redis by ForgotPassword) and updates
// the player's password directly in the player DB. Bypasses the player service gRPC which
// would try to re-verify the OTP in its own Redis (a different store).
func (h *OTPHandler) ResetPassword(w http.ResponseWriter, r *http.Request) error {
	var req resetPasswordRequest
	if err := router.ReadJSON(r, &req); err != nil {
		return router.ErrorResponse(w, http.StatusBadRequest, "Invalid request body")
	}
	if req.PhoneNumber == "" || req.Code == "" || req.NewPassword == "" {
		return router.ErrorResponse(w, http.StatusBadRequest, "phone_number, code and new_password are required")
	}
	if len(req.NewPassword) < 6 {
		return router.ErrorResponse(w, http.StatusBadRequest, "Password must be at least 6 characters")
	}

	phone := toE164(req.PhoneNumber)
	redisKey := fmt.Sprintf("pwd_reset_otp:%s", phone)

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	stored, err := h.redis.Get(ctx, redisKey).Result()
	if err == redis.Nil {
		return router.ErrorResponse(w, http.StatusBadRequest, "Code expired or not found. Please request a new one.")
	}
	if err != nil {
		return router.ErrorResponse(w, http.StatusInternalServerError, "Verification failed")
	}

	parts := strings.SplitN(stored, ":", 2)
	if len(parts) != 2 || parts[0] != strings.TrimSpace(req.Code) {
		return router.ErrorResponse(w, http.StatusBadRequest, "Invalid code")
	}
	playerID := parts[1]

	playerDSN := os.Getenv("PLAYER_DB_DSN")
	if playerDSN == "" {
		playerDSN = "host=service-player-db port=5432 user=player password=#yerpla@333! dbname=player_service sslmode=disable"
	}
	pdb, err := sql.Open("postgres", playerDSN)
	if err != nil {
		return router.ErrorResponse(w, http.StatusInternalServerError, "Failed to update password")
	}
	defer pdb.Close()

	dbCtx, dbCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer dbCancel()

	var currentHash string
	_ = pdb.QueryRowContext(dbCtx, `SELECT password_hash FROM players WHERE id = $1 LIMIT 1`, playerID).Scan(&currentHash)

	// Check before consuming OTP so user can retry with a different password
	if currentHash != "" {
		if bcrypt.CompareHashAndPassword([]byte(currentHash), []byte(req.NewPassword)) == nil {
			return router.ErrorResponse(w, http.StatusBadRequest, "Please choose a different password from your current one")
		}
	}

	// New password is different — consume the OTP to prevent replay
	h.redis.Del(ctx, redisKey)

	newHash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		return router.ErrorResponse(w, http.StatusInternalServerError, "Failed to update password")
	}

	_, err = pdb.ExecContext(dbCtx,
		`UPDATE players SET password_hash = $1, updated_at = NOW() WHERE id = $2`,
		string(newHash), playerID,
	)
	if err != nil {
		return router.ErrorResponse(w, http.StatusInternalServerError, "Failed to update password")
	}

	return router.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Password updated successfully. You can now sign in.",
	})
}
