package entity

import (
	"strings"
	"time"
)

// StationState represents the connection state of a station
type StationState string

const (
	StateOnline  StationState = "ONLINE"
	StateOffline StationState = "OFFLINE"
	StateUnknown StationState = "UNKNOWN"
)

// StationUptime holds uptime/downtime statistics for a station over a period
type StationUptime struct {
	ChargePointId   string        `json:"charge_point_id" bson:"charge_point_id"`
	OnlineDuration  time.Duration `json:"-" bson:"-"`
	OfflineDuration time.Duration `json:"-" bson:"-"`
	UptimePercent   float64       `json:"uptime_percent" bson:"uptime_percent"`
	FinalState      StationState  `json:"final_state" bson:"final_state"`
}

// StationUptimeJSON is the JSON-friendly representation for API responses
type StationUptimeJSON struct {
	ChargePointId  string  `json:"charge_point_id"`
	OnlineSeconds  int64   `json:"online_seconds"`
	OfflineSeconds int64   `json:"offline_seconds"`
	OnlineMinutes  float64 `json:"online_minutes"`
	OfflineMinutes float64 `json:"offline_minutes"`
	UptimePercent  float64 `json:"uptime_percent"`
	FinalState     string  `json:"final_state"`
}

// ToJSON converts StationUptime to JSON-friendly format
func (s *StationUptime) ToJSON() StationUptimeJSON {
	return StationUptimeJSON{
		ChargePointId:  s.ChargePointId,
		OnlineSeconds:  int64(s.OnlineDuration.Seconds()),
		OfflineSeconds: int64(s.OfflineDuration.Seconds()),
		OnlineMinutes:  s.OnlineDuration.Minutes(),
		OfflineMinutes: s.OfflineDuration.Minutes(),
		UptimePercent:  s.UptimePercent,
		FinalState:     string(s.FinalState),
	}
}

// StationStatus holds current connection state for a station
type StationStatus struct {
	ChargePointId string        `json:"charge_point_id" bson:"charge_point_id"`
	State         StationState  `json:"state" bson:"state"`
	Since         time.Time     `json:"since" bson:"since"`
	Duration      time.Duration `json:"-" bson:"-"`
	LastEventText string        `json:"last_event_text,omitempty" bson:"last_event_text,omitempty"`
}

// StationStatusJSON is the JSON-friendly representation for API responses
type StationStatusJSON struct {
	ChargePointId   string  `json:"charge_point_id"`
	State           string  `json:"state"`
	Since           string  `json:"since"`
	DurationSeconds int64   `json:"duration_seconds"`
	DurationMinutes float64 `json:"duration_minutes"`
	LastEventText   string  `json:"last_event_text,omitempty"`
}

// ToJSON converts StationStatus to JSON-friendly format
func (s *StationStatus) ToJSON() StationStatusJSON {
	return StationStatusJSON{
		ChargePointId:   s.ChargePointId,
		State:           string(s.State),
		Since:           s.Since.Format(time.RFC3339),
		DurationSeconds: int64(s.Duration.Seconds()),
		DurationMinutes: s.Duration.Minutes(),
		LastEventText:   s.LastEventText,
	}
}

// StateFromText maps event text to station state
// "registered" -> ONLINE, "unregistered" -> OFFLINE
func StateFromText(text string) StationState {
	textLower := strings.ToLower(text)
	// Check for "unregistered" first (it contains "registered")
	if strings.Contains(textLower, "unregistered") {
		return StateOffline
	}
	// Check for "registered" (online)
	if strings.Contains(textLower, "registered") {
		return StateOnline
	}
	return StateUnknown
}
