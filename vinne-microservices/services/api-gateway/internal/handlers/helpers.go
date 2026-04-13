package handlers

import (
	"encoding/json"
	"net/http"
	"strings"

	agentmgmtpb "github.com/randco/randco-microservices/proto/agent/management/v1"
)

// convertStatusStringToEnum converts status string to EntityStatus enum
func convertStatusStringToEnum(status string) agentmgmtpb.EntityStatus {
	switch strings.ToLower(status) {
	case "active":
		return agentmgmtpb.EntityStatus_ENTITY_STATUS_ACTIVE
	case "suspended":
		return agentmgmtpb.EntityStatus_ENTITY_STATUS_SUSPENDED
	case "under_review":
		return agentmgmtpb.EntityStatus_ENTITY_STATUS_UNDER_REVIEW
	case "terminated":
		return agentmgmtpb.EntityStatus_ENTITY_STATUS_TERMINATED
	case "inactive":
		return agentmgmtpb.EntityStatus_ENTITY_STATUS_INACTIVE
	case "pending":
		return agentmgmtpb.EntityStatus_ENTITY_STATUS_PENDING
	default:
		return agentmgmtpb.EntityStatus_ENTITY_STATUS_UNSPECIFIED
	}
}

// convertStatusEnumToString converts EntityStatus enum to clean string for API response
func convertStatusEnumToString(status agentmgmtpb.EntityStatus) string {
	switch status {
	case agentmgmtpb.EntityStatus_ENTITY_STATUS_ACTIVE:
		return "ACTIVE"
	case agentmgmtpb.EntityStatus_ENTITY_STATUS_SUSPENDED:
		return "SUSPENDED"
	case agentmgmtpb.EntityStatus_ENTITY_STATUS_UNDER_REVIEW:
		return "UNDER_REVIEW"
	case agentmgmtpb.EntityStatus_ENTITY_STATUS_TERMINATED:
		return "TERMINATED"
	case agentmgmtpb.EntityStatus_ENTITY_STATUS_INACTIVE:
		return "INACTIVE"
	case agentmgmtpb.EntityStatus_ENTITY_STATUS_PENDING:
		return "PENDING"
	case agentmgmtpb.EntityStatus_ENTITY_STATUS_UNSPECIFIED:
		return "UNSPECIFIED"
	default:
		return "UNKNOWN"
	}
}

// convertDeviceStatusToString converts DeviceStatus enum to string for API response
func convertDeviceStatusToString(status agentmgmtpb.DeviceStatus) string {
	switch status {
	case agentmgmtpb.DeviceStatus_DEVICE_STATUS_ASSIGNED:
		return "Active"
	case agentmgmtpb.DeviceStatus_DEVICE_STATUS_AVAILABLE:
		return "Inactive"
	case agentmgmtpb.DeviceStatus_DEVICE_STATUS_MAINTENANCE:
		return "Faulty"
	case agentmgmtpb.DeviceStatus_DEVICE_STATUS_RETIRED:
		return "Inactive"
	case agentmgmtpb.DeviceStatus_DEVICE_STATUS_UNSPECIFIED:
		return "Inactive"
	default:
		return "Inactive"
	}
}

// convertKYCStatusToString converts KYCStatus enum to string for API response
func convertKYCStatusToString(status agentmgmtpb.KYCStatus) string {
	switch status {
	case agentmgmtpb.KYCStatus_KYC_STATUS_APPROVED:
		return "APPROVED"
	case agentmgmtpb.KYCStatus_KYC_STATUS_PENDING:
		return "PENDING"
	case agentmgmtpb.KYCStatus_KYC_STATUS_REJECTED:
		return "REJECTED"
	case agentmgmtpb.KYCStatus_KYC_STATUS_EXPIRED:
		return "EXPIRED"
	case agentmgmtpb.KYCStatus_KYC_STATUS_UNSPECIFIED:
		return "UNSPECIFIED"
	default:
		return "UNKNOWN"
	}
}

// parseRequestBody parses JSON request body into the provided struct
func parseRequestBody(r *http.Request, v interface{}) error {
	return json.NewDecoder(r.Body).Decode(v)
}
