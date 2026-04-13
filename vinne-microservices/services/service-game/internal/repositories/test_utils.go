package repositories

import "time"

// stringPtr returns a pointer to the given string value
func stringPtr(s string) *string {
	return &s
}

// int32Ptr returns a pointer to the given int32 value
func int32Ptr(i int32) *int32 {
	return &i
}

// boolPtr returns a pointer to the given bool value
func boolPtr(b bool) *bool {
	return &b
}

// timePtr returns a pointer to the given time value
func timePtr(t time.Time) *time.Time {
	return &t
}
