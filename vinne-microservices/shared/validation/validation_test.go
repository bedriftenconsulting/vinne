package validation

import (
	"testing"
)

func TestValidateAgentCode(t *testing.T) {
	tests := []struct {
		name    string
		code    string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid 4-digit code",
			code:    "1001",
			wantErr: false,
		},
		{
			name:    "valid 5-digit code",
			code:    "12345",
			wantErr: false,
		},
		{
			name:    "valid 8-digit code",
			code:    "12345678",
			wantErr: false,
		},
		{
			name:    "empty code",
			code:    "",
			wantErr: true,
			errMsg:  "agent code is required",
		},
		{
			name:    "code with letters",
			code:    "AG1001",
			wantErr: true,
			errMsg:  "invalid agent code format (expected format: 1001)",
		},
		{
			name:    "valid 3-digit code",
			code:    "123",
			wantErr: false,
		},
		{
			name:    "valid 9-digit code",
			code:    "123456789",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateAgentCode(tt.code)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateAgentCode() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil && tt.errMsg != "" {
				if err.Error() != "agent_code: "+tt.errMsg {
					t.Errorf("ValidateAgentCode() error = %v, want %v", err.Error(), "agent_code: "+tt.errMsg)
				}
			}
		})
	}
}

func TestValidateRetailerCode(t *testing.T) {
	tests := []struct {
		name    string
		code    string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid 8-digit retailer code",
			code:    "12345678",
			wantErr: false,
		},
		{
			name:    "valid agent-managed retailer code",
			code:    "10011234",
			wantErr: false,
		},
		{
			name:    "empty code",
			code:    "",
			wantErr: true,
			errMsg:  "retailer code is required",
		},
		{
			name:    "code too short",
			code:    "1001",
			wantErr: true,
			errMsg:  "invalid retailer code format (expected format: 12345678)",
		},
		{
			name:    "code with letters",
			code:    "RT123456",
			wantErr: true,
			errMsg:  "invalid retailer code format (expected format: 12345678)",
		},
		{
			name:    "code too long",
			code:    "123456789",
			wantErr: true,
			errMsg:  "invalid retailer code format (expected format: 12345678)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateRetailerCode(tt.code)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateRetailerCode() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil && tt.errMsg != "" {
				if err.Error() != "retailer_code: "+tt.errMsg {
					t.Errorf("ValidateRetailerCode() error = %v, want %v", err.Error(), "retailer_code: "+tt.errMsg)
				}
			}
		})
	}
}
