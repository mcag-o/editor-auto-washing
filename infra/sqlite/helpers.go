package sqlite

import "time"

func nullableInt(value *int) any {
	if value == nil {
		return nil
	}
	return *value
}

func emptyToNil(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func nullableTimeNano(value *time.Time) any {
	if value == nil {
		return nil
	}
	return value.Format(time.RFC3339Nano)
}
