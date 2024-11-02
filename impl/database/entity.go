package database

type ReportLineMonth struct {
	Year    int    `json:"year"`
	Month   int    `json:"month"`
	Total   int    `json:"totalConsumed"`
	Count   int    `json:"count"`
	Average string `json:"avgWatts"`
}
