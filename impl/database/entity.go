package database

type ReportLineMonth struct {
	Year    int `json:"year"`
	Month   int `json:"month"`
	Total   int `json:"totalConsumedWatts"`
	Count   int `json:"count"`
	Average int `json:"avgWatts"`
}
