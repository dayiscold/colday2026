package models

type Release struct {
	ID           int64  `json:"id"`
	Title        string `json:"title"`
	ReleaseType  string `json:"release_type"`
	Cover        string `json:"cover"`
	Bandlink     string `json:"bandlink"`
	UPC          string `json:"upc"`
	OriginalDate string `json:"original_date"`
	CounterSongs int    `json:"counter_songs"`
	SongDuration string `json:"song_duration"`
	Featured     bool   `json:"featured"`
}
