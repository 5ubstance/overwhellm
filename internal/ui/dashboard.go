package ui

import (
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"strings"
	"time"

	"overwhellm/internal/db"
)

type Dashboard struct {
	db *db.DB
}

func New(db *db.DB) *Dashboard {
	return &Dashboard{db: db}
}

func (d *Dashboard) SetupMux(mux *http.ServeMux) {
	mux.HandleFunc("/", d.handleIndex)
	mux.HandleFunc("/api/metrics/summary", d.handleSummary)
	mux.HandleFunc("/api/metrics/trends", d.handleTrends)
	mux.HandleFunc("/api/requests", d.handleRequests)
	mux.HandleFunc("/api/requests/", d.handleRequestDetail)
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("./internal/ui/static"))))
}

func (d *Dashboard) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	stats, err := d.db.GetSummaryStats()
	if err != nil {
		http.Error(w, "Failed to get stats", http.StatusInternalServerError)
		return
	}

	tmpl := template.Must(template.ParseFiles("./internal/ui/templates/metrics.html"))

	data := struct {
		Stats       map[string]interface{}
		CurrentTime time.Time
	}{
		Stats:       stats,
		CurrentTime: time.Now(),
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.Execute(w, data); err != nil {
		http.Error(w, "Failed to render template", http.StatusInternalServerError)
	}
}

func (d *Dashboard) handleSummary(w http.ResponseWriter, r *http.Request) {
	stats, err := d.db.GetSummaryStats()
	if err != nil {
		http.Error(w, "Failed to get stats", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(stats); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

func (d *Dashboard) handleTrends(w http.ResponseWriter, r *http.Request) {
	interval := r.URL.Query().Get("interval")
	if interval == "" {
		interval = "hour"
	}

	end := time.Now()
	var start time.Time

	switch interval {
	case "hour":
		start = end.Add(-24 * time.Hour)
	case "day":
		start = end.Add(-7 * 24 * time.Hour)
	case "week":
		start = end.Add(-30 * 24 * time.Hour)
	default:
		start = end.Add(-24 * time.Hour)
	}

	trends, err := d.db.GetTrends(start, end, interval)
	if err != nil {
		http.Error(w, "Failed to get trends", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(trends); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

func (d *Dashboard) handleRequests(w http.ResponseWriter, r *http.Request) {
	limit := 50
	offset := 0

	if l := r.URL.Query().Get("limit"); l != "" {
		fmt.Sscanf(l, "%d", &limit)
	}
	if o := r.URL.Query().Get("offset"); o != "" {
		fmt.Sscanf(o, "%d", &offset)
	}

	requests, err := d.db.GetRecentRequests(limit, offset)
	if err != nil {
		http.Error(w, "Failed to get requests", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(requests); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

func (d *Dashboard) handleRequestDetail(w http.ResponseWriter, r *http.Request) {
	// Extract ID from path
	path := strings.TrimPrefix(r.URL.Path, "/api/requests/")
	id := strings.Split(path, "?")[0] // Remove query params

	req, err := d.db.GetRequest(id)
	if err != nil {
		http.Error(w, "Request not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(req); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}
