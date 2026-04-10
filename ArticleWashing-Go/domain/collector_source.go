package domain

import "time"

const (
	CollectorAuthModeNone   = "none"
	CollectorAuthModeCookie = "cookie"
	CollectorAuthModeHeader = "header"
)

type CollectorSource struct {
	ID                 string         `json:"id"`
	DisplayName        string         `json:"display_name"`
	Enabled            bool           `json:"enabled"`
	ScheduleEnabled    bool           `json:"schedule_enabled"`
	IntervalMinutes    int            `json:"interval_minutes"`
	AuthMode           string         `json:"auth_mode"`
	TimeoutMS          int            `json:"timeout_ms"`
	HeadersJSON        []byte         `json:"headers_json"`
	CookieSecretRef    string         `json:"cookie_secret_ref"`
	HeaderSecretRef    string         `json:"header_secret_ref"`
	HotlistLimit       int            `json:"hotlist_limit"`
	DetailFetchEnabled bool           `json:"detail_fetch_enabled"`
	Concurrency        int            `json:"concurrency"`
	RetryPolicyJSON    []byte         `json:"retry_policy_json"`
	OptionsJSON        []byte         `json:"options_json"`
	Metadata           map[string]any `json:"metadata"`
	CreatedAt          time.Time      `json:"created_at"`
	UpdatedAt          time.Time      `json:"updated_at"`
}

func NewCollectorSource(sourceID, displayName string) *CollectorSource {
	now := time.Now().UTC()
	return &CollectorSource{
		ID:                 sourceID,
		DisplayName:        displayName,
		Enabled:            true,
		ScheduleEnabled:    true,
		IntervalMinutes:    30,
		AuthMode:           CollectorAuthModeNone,
		TimeoutMS:          10000,
		HotlistLimit:       50,
		DetailFetchEnabled: false,
		Concurrency:        1,
		HeadersJSON:        []byte("{}"),
		RetryPolicyJSON:    []byte("{}"),
		OptionsJSON:        []byte("{}"),
		Metadata:           map[string]any{},
		CreatedAt:          now,
		UpdatedAt:          now,
	}
}
