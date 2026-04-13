package services

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/randco/service-player/internal/models"
	"github.com/randco/service-player/internal/repositories"
)

type sessionService struct {
	sessionRepo repositories.SessionRepository
	deviceRepo  repositories.DeviceRepository
}

func NewSessionService(
	sessionRepo repositories.SessionRepository,
	deviceRepo repositories.DeviceRepository,
) SessionService {
	return &sessionService{
		sessionRepo: sessionRepo,
		deviceRepo:  deviceRepo,
	}
}

func (s *sessionService) CreateSession(ctx context.Context, playerID uuid.UUID, deviceID, channel string) (*models.PlayerSession, error) {
	device, err := s.deviceRepo.GetByPlayerAndDeviceID(ctx, playerID, deviceID)
	if err != nil {
		deviceReq := models.CreateDeviceRequest{
			PlayerID:   playerID,
			DeviceID:   deviceID,
			DeviceType: "mobile", // TODO: Get from context
			DeviceName: "Unknown Device",
			OS:         "Unknown OS",
			OSVersion:  "Unknown Version",
			AppVersion: "Unknown Version",
			TrustScore: 50, // Default trust score
		}

		device, err = s.deviceRepo.Create(ctx, deviceReq)
		if err != nil {
			return nil, fmt.Errorf("failed to create device: %w", err)
		}
	}

	now := time.Now()
	sessionReq := models.CreateSessionRequest{
		PlayerID:     playerID,
		DeviceID:     deviceID,
		RefreshToken: uuid.New().String(),
		Channel:      channel,
		DeviceType:   device.DeviceType,
		AppVersion:   device.AppVersion,
		ExpiresAt:    now.Add(7 * 24 * time.Hour), // 7 days
	}

	session, err := s.sessionRepo.Create(ctx, sessionReq)
	if err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	deviceUpdateReq := models.UpdateDeviceRequest{
		ID: device.ID,
	}

	_, err = s.deviceRepo.Update(ctx, deviceUpdateReq)
	if err != nil {
		fmt.Printf("Failed to update device last seen: %v\n", err)
	}

	return session, nil
}

func (s *sessionService) GetSession(ctx context.Context, sessionID uuid.UUID) (*models.PlayerSession, error) {
	session, err := s.sessionRepo.GetByID(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("session not found: %w", err)
	}

	return session, nil
}

func (s *sessionService) UpdateSession(ctx context.Context, sessionID uuid.UUID, updates models.UpdateSessionRequest) (*models.PlayerSession, error) {
	// Set the session ID in the update request
	updates.ID = sessionID

	session, err := s.sessionRepo.Update(ctx, updates)
	if err != nil {
		return nil, fmt.Errorf("failed to update session: %w", err)
	}

	return session, nil
}

func (s *sessionService) RevokeSession(ctx context.Context, sessionID uuid.UUID, reason string) error {
	now := time.Now()
	updateReq := models.UpdateSessionRequest{
		ID:            sessionID,
		IsActive:      false,
		RevokedAt:     now,
		RevokedReason: reason,
	}

	_, err := s.sessionRepo.Update(ctx, updateReq)
	if err != nil {
		return fmt.Errorf("failed to revoke session: %w", err)
	}

	return nil
}

func (s *sessionService) GetPlayerSessions(ctx context.Context, playerID uuid.UUID) ([]*models.PlayerSession, error) {
	sessions, err := s.sessionRepo.GetByPlayerID(ctx, playerID)
	if err != nil {
		return nil, fmt.Errorf("failed to get player sessions: %w", err)
	}

	var activeSessions []*models.PlayerSession
	for _, session := range sessions {
		if session.IsActive && time.Now().Before(session.ExpiresAt) {
			activeSessions = append(activeSessions, session)
		}
	}

	return activeSessions, nil
}

func (s *sessionService) CleanupExpiredSessions(ctx context.Context) error {
	// This would typically be called by a background job
	// For now, we'll implement a simple cleanup

	// Get all sessions (this is not efficient for large datasets)
	// In production, you'd want to query only expired sessions
	// sessions, err := s.sessionRepo.List(ctx, models.SessionFilter{})
	// if err != nil {
	//     return fmt.Errorf("failed to get sessions: %w", err)
	// }

	// now := time.Now()
	// for _, session := range sessions {
	//     if now.After(session.ExpiresAt) {
	//         err := s.RevokeSession(ctx, session.ID, "expired")
	//         if err != nil {
	//             fmt.Printf("Failed to revoke expired session %s: %v\n", session.ID, err)
	//         }
	//     }
	// }

	return nil
}
