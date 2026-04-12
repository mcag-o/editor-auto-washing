package sqlite

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
