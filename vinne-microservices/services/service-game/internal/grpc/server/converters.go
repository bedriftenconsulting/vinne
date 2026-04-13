package server

import (
	pb "github.com/randco/randco-microservices/proto/game/v1"
	"github.com/randco/randco-microservices/services/service-game/internal/models"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// convertGameToProto converts a models.Game to pb.Game
func convertGameToProto(game *models.Game) *pb.Game {
	if game == nil {
		return nil
	}

	// Convert to the new proto format matching the frontend expectations
	return &pb.Game{
		Id:                  game.ID.String(),
		Code:                game.Code,
		Name:                game.Name,
		Organizer:           game.Organizer,
		GameCategory:        game.GameCategory,
		GameFormat:          game.GameFormat,
		GameType:            convertStringPtrToProto(game.GameType),
		NumberRangeMin:      game.NumberRangeMin,
		NumberRangeMax:      game.NumberRangeMax,
		SelectionCount:      game.SelectionCount,
		StartTime:           convertStringPtrToProto(game.StartTimeStr),
		EndTime:             convertStringPtrToProto(game.EndTimeStr),
		DrawFrequency:       game.DrawFrequency,
		DrawDays:            game.DrawDays,
		DrawTime:            convertStringPtrToProto(game.DrawTimeStr),
		SalesCutoffMinutes:  game.SalesCutoffMinutes,
		MinStake:            game.MinStakeAmount,
		MaxStake:            game.MaxStakeAmount,
		BasePrice:           game.BasePrice,
		MaxTicketsPerPlayer: game.MaxTicketsPerPlayer,
		MultiDrawEnabled:    game.MultiDrawEnabled,
		MaxDrawsAdvance:     int32PtrToInt32(game.MaxDrawsAdvance),
		WeeklySchedule:      boolPtrToBool(game.WeeklySchedule),
		Description:         convertStringPtrToProto(game.Description),
		Status:              game.Status,
		Version:             game.Version,
		LogoUrl:             convertStringPtrToProto(game.LogoURL),
		BrandColor:          convertStringPtrToProto(game.BrandColor),
		CreatedAt:           timestamppb.New(game.CreatedAt),
		UpdatedAt:           timestamppb.New(game.UpdatedAt),
	}
}

// convertProtoStringPtr converts string to *string (for optional fields)
// Returns nil only for empty strings, allowing validation to happen downstream
func convertProtoStringPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// convertProtoTimeStringPtr converts string to *string for time fields
// This is separate from convertProtoStringPtr to make time handling explicit
// Returns nil for empty strings (time not provided)
func convertProtoTimeStringPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// convertProtoOrganizerFilter converts string organizer to *models.Organizer for filtering
func convertProtoOrganizerFilter(protoOrg string) *models.Organizer {
	if protoOrg == "" {
		return nil
	}
	switch protoOrg {
	case "ORGANIZER_NLA":
		org := models.OrganizerNLA
		return &org
	case "ORGANIZER_RAND_LOTTERY":
		org := models.OrganizerRandLottery
		return &org
	default:
		return nil
	}
}

// convertProtoStatusFilter converts string status to *models.GameStatus for filtering
func convertProtoStatusFilter(protoStatus string) *models.GameStatus {
	if protoStatus == "" {
		return nil
	}
	switch protoStatus {
	case "DRAFT":
		status := models.GameStatusDraft
		return &status
	case "PENDING_APPROVAL":
		status := models.GameStatusPendingApproval
		return &status
	case "APPROVED":
		status := models.GameStatusApproved
		return &status
	case "ACTIVE":
		status := models.GameStatusActive
		return &status
	case "SUSPENDED":
		status := models.GameStatusSuspended
		return &status
	case "TERMINATED":
		status := models.GameStatusTerminated
		return &status
	}
	return nil
}

// convertStringPtrToProto converts *string to string for proto
func convertStringPtrToProto(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// convertProtoBetTypes converts proto BetTypes to model BetTypes
func convertProtoBetTypes(protoBetTypes []*pb.BetType) []models.BetType {
	var betTypes []models.BetType
	for _, bt := range protoBetTypes {
		if bt != nil {
			betTypes = append(betTypes, models.BetType{
				ID:         bt.Id,
				Name:       bt.Name,
				Enabled:    bt.Enabled,
				Multiplier: float64(bt.Multiplier),
			})
		}
	}
	return betTypes
}

// convertProtoInt32Ptr converts int32 to *int32 (for optional fields)
// Note: This returns the pointer even for zero values because in protobuf,
// zero is a valid value and different from "not set". However, since proto3
// doesn't have optional scalars by default, we treat zero as "not set".
func convertProtoInt32Ptr(i int32) *int32 {
	if i == 0 {
		return nil
	}
	return &i
}

// convertProtoBoolPtr converts bool to *bool (for optional fields)
func convertProtoBoolPtr(b bool) *bool {
	if !b {
		return nil
	}
	return &b
}

// int32PtrToInt32 converts *int32 to int32 (returns 0 if nil)
func int32PtrToInt32(i *int32) int32 {
	if i == nil {
		return 0
	}
	return *i
}

// boolPtrToBool converts *bool to bool (returns false if nil)
func boolPtrToBool(b *bool) bool {
	if b == nil {
		return false
	}
	return *b
}
