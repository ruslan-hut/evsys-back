package database

type ReportLineMonth struct {
	Year    int   `json:"year"`
	Month   int   `json:"month"`
	Total   int64 `json:"totalConsumed"`
	Count   int64 `json:"count"`
	Average int64 `json:"avgWatts"`
}
