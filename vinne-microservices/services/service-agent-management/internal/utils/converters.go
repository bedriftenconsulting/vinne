package utils

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// StringToUUID converts a string to UUID with error handling
func StringToUUID(s string) (uuid.UUID, error) {
	if s == "" {
		return uuid.Nil, nil
	}
	return uuid.Parse(s)
}

// UUIDToString converts a UUID to string
func UUIDToString(u uuid.UUID) string {
	if u == uuid.Nil {
		return ""
	}
	return u.String()
}

// UUIDPtrToString converts a UUID pointer to string
func UUIDPtrToString(u *uuid.UUID) string {
	if u == nil || *u == uuid.Nil {
		return ""
	}
	return u.String()
}

// StringToUUIDPtr converts a string to UUID pointer
func StringToUUIDPtr(s string) *uuid.UUID {
	if s == "" {
		return nil
	}
	id, err := uuid.Parse(s)
	if err != nil {
		return nil
	}
	return &id
}

// TimeToTimestamp converts time.Time to protobuf timestamp
func TimeToTimestamp(t time.Time) *timestamppb.Timestamp {
	if t.IsZero() {
		return nil
	}
	return timestamppb.New(t)
}

// TimestampToTime converts protobuf timestamp to time.Time
func TimestampToTime(ts *timestamppb.Timestamp) time.Time {
	if ts == nil {
		return time.Time{}
	}
	return ts.AsTime()
}

// StringPtr returns a pointer to a string
func StringPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// StringValue returns the value of a string pointer
func StringValue(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// Int32Ptr returns a pointer to an int32
func Int32Ptr(i int32) *int32 {
	return &i
}

// Int32Value returns the value of an int32 pointer
func Int32Value(i *int32) int32 {
	if i == nil {
		return 0
	}
	return *i
}

// BoolPtr returns a pointer to a bool
func BoolPtr(b bool) *bool {
	return &b
}

// BoolValue returns the value of a bool pointer
func BoolValue(b *bool) bool {
	if b == nil {
		return false
	}
	return *b
}

// TimePtr returns a pointer to time.Time
func TimePtr(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	return &t
}

// TimeValue returns the value of a time.Time pointer
func TimeValue(t *time.Time) time.Time {
	if t == nil {
		return time.Time{}
	}
	return *t
}

// GenerateCode generates a unique code with prefix
func GenerateCode(prefix string, sequence int) string {
	return fmt.Sprintf("%s%06d", prefix, sequence)
}

// TruncateString truncates a string to a maximum length
func TruncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

// SafeSubstring returns a substring safely without panicking
func SafeSubstring(s string, start, end int) string {
	if start < 0 {
		start = 0
	}
	if end > len(s) {
		end = len(s)
	}
	if start >= end {
		return ""
	}
	return s[start:end]
}
