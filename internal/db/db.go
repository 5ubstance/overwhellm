package db

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/google/uuid"
)

type Request struct {
	ID                string
	ClientIP          string
	UserAgent         string
	Endpoint          string
	Model             string
	Method            string
	StatusCode        int
	DurationMs        int
	TTFTMs            int
	TokensInput       int
	TokensOutput      int
	RequestSizeBytes  int
	ResponseSizeBytes int
	CreatedAt         time.Time
}

type DB struct {
	filePath string
	mu       sync.RWMutex
	requests map[string]*Request
}

func New(dbPath string) (*DB, error) {
	db := &DB{
		filePath: dbPath,
		requests: make(map[string]*Request),
	}

	// Load existing data
	if err := db.load(); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("failed to load database: %w", err)
	}

	return db, nil
}

func (db *DB) load() error {
	data, err := os.ReadFile(db.filePath)
	if err != nil {
		return err
	}

	var requests []*Request
	if err := json.Unmarshal(data, &requests); err != nil {
		return err
	}

	for _, req := range requests {
		db.requests[req.ID] = req
	}

	return nil
}

func (db *DB) save() error {
	db.mu.Lock()
	defer db.mu.Unlock()

	var requests []*Request
	for _, req := range db.requests {
		requests = append(requests, req)
	}

	data, err := json.MarshalIndent(requests, "", "  ")
	if err != nil {
		fmt.Printf("[DB] Failed to marshal: %v\n", err)
		return err
	}

	err = os.WriteFile(db.filePath, data, 0644)
	if err != nil {
		fmt.Printf("[DB] Failed to write file: %v\n", err)
		return err
	}

	fmt.Printf("[DB] Saved %d requests to %s\n", len(requests), db.filePath)
	return nil
}

func (db *DB) CreateRequest(req *Request) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	req.ID = uuid.New().String()
	req.CreatedAt = time.Now()

	db.requests[req.ID] = req

	return db.save()
}

func (db *DB) GetRequest(id string) (*Request, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	req, ok := db.requests[id]
	if !ok {
		return nil, fmt.Errorf("request not found")
	}

	// Return a copy
	reqCopy := *req
	return &reqCopy, nil
}

func (db *DB) GetRecentRequests(limit, offset int) ([]*Request, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	var requests []*Request
	for _, req := range db.requests {
		requests = append(requests, req)
	}

	// Sort by created_at descending
	for i := 0; i < len(requests)-1; i++ {
		for j := i + 1; j < len(requests); j++ {
			if requests[j].CreatedAt.After(requests[i].CreatedAt) {
				requests[i], requests[j] = requests[j], requests[i]
			}
		}
	}

	if offset >= len(requests) {
		return []*Request{}, nil
	}

	end := offset + limit
	if end > len(requests) {
		end = len(requests)
	}

	result := make([]*Request, end-offset)
	copy(result, requests[offset:end])

	return result, nil
}

func (db *DB) GetTotalRequests() (int64, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	return int64(len(db.requests)), nil
}

func (db *DB) GetRequestsByDateRange(start, end time.Time) ([]*Request, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	var requests []*Request
	for _, req := range db.requests {
		if !req.CreatedAt.Before(start) && !req.CreatedAt.After(end) {
			requests = append(requests, req)
		}
	}

	return requests, nil
}

func (db *DB) GetSummaryStats() (map[string]interface{}, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	stats := make(map[string]interface{})

	requests := make([]*Request, 0, len(db.requests))
	for _, req := range db.requests {
		requests = append(requests, req)
	}

	// Total requests
	stats["total"] = len(requests)

	// Today's stats
	today := time.Now().Truncate(24 * time.Hour)
	var todayTotal, todayTokensIn, todayTokensOut, todayLatencySum int64
	var todayRequests []*Request

	for _, req := range requests {
		if !req.CreatedAt.Before(today) {
			todayRequests = append(todayRequests, req)
			todayTotal++
			todayTokensIn += int64(req.TokensInput)
			todayTokensOut += int64(req.TokensOutput)
			todayLatencySum += int64(req.DurationMs)
		}
	}

	stats["today_total"] = todayTotal
	stats["today_tokens_in"] = todayTokensIn
	stats["today_tokens_out"] = todayTokensOut
	if todayTotal > 0 {
		stats["today_avg_latency_ms"] = float64(todayLatencySum) / float64(todayTotal)
	}

	// This week's stats
	week := time.Now().AddDate(0, 0, -7).Truncate(24 * time.Hour)
	var weekTotal, weekTokensIn, weekTokensOut int64
	for _, req := range requests {
		if !req.CreatedAt.Before(week) {
			weekTotal++
			weekTokensIn += int64(req.TokensInput)
			weekTokensOut += int64(req.TokensOutput)
		}
	}
	stats["week_total"] = weekTotal
	stats["week_tokens_in"] = weekTokensIn
	stats["week_tokens_out"] = weekTokensOut

	// This month's stats
	month := time.Now().AddDate(0, -1, 0).Truncate(24 * time.Hour)
	var monthTotal, monthTokensIn, monthTokensOut int64
	for _, req := range requests {
		if !req.CreatedAt.Before(month) {
			monthTotal++
			monthTokensIn += int64(req.TokensInput)
			monthTokensOut += int64(req.TokensOutput)
		}
	}
	stats["month_total"] = monthTotal
	stats["month_tokens_in"] = monthTokensIn
	stats["month_tokens_out"] = monthTokensOut

	// Average tokens per request
	var totalTokensIn, totalTokensOut int64
	var totalLatency int64
	for _, req := range requests {
		totalTokensIn += int64(req.TokensInput)
		totalTokensOut += int64(req.TokensOutput)
		totalLatency += int64(req.DurationMs)
	}

	if len(requests) > 0 {
		stats["avg_tokens_in"] = float64(totalTokensIn) / float64(len(requests))
		stats["avg_tokens_out"] = float64(totalTokensOut) / float64(len(requests))
		stats["avg_latency_ms"] = float64(totalLatency) / float64(len(requests))

		// Token throughput
		totalDuration := float64(totalLatency) / 1000.0
		if totalDuration > 0 {
			stats["tokens_per_second"] = float64(totalTokensIn+totalTokensOut) / totalDuration
		}
	} else {
		stats["avg_tokens_in"] = 0
		stats["avg_tokens_out"] = 0
		stats["avg_latency_ms"] = 0
		stats["tokens_per_second"] = 0
	}

	return stats, nil
}

func (db *DB) GetTrends(start, end time.Time, interval string) (map[string]interface{}, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	trends := make(map[string]interface{})
	var labels []string
	var dataPoints []map[string]interface{}

	requests := make([]*Request, 0, len(db.requests))
	for _, req := range db.requests {
		if !req.CreatedAt.Before(start) && !req.CreatedAt.After(end) {
			requests = append(requests, req)
		}
	}

	// Group by time period
	type periodKey struct {
		period string
	}
	periods := make(map[periodKey]*struct {
		count     int64
		latency   int64
		tokensIn  int64
		tokensOut int64
	})

	for _, req := range requests {
		var period string
		switch interval {
		case "hour":
			period = req.CreatedAt.Truncate(time.Hour).Format("2006-01-02 15:00:00")
		case "day":
			period = req.CreatedAt.Format("2006-01-02")
		default:
			period = req.CreatedAt.Format("2006-01-02")
		}

		key := periodKey{period: period}
		if p, ok := periods[key]; ok {
			p.count++
			p.latency += int64(req.DurationMs)
			p.tokensIn += int64(req.TokensInput)
			p.tokensOut += int64(req.TokensOutput)
		} else {
			periods[key] = &struct {
				count     int64
				latency   int64
				tokensIn  int64
				tokensOut int64
			}{
				count:     1,
				latency:   int64(req.DurationMs),
				tokensIn:  int64(req.TokensInput),
				tokensOut: int64(req.TokensOutput),
			}
		}
	}

	// Sort periods and create data points
	for key, data := range periods {
		labels = append(labels, key.period)
		dataPoints = append(dataPoints, map[string]interface{}{
			"count":       data.count,
			"avg_latency": float64(data.latency) / float64(data.count),
			"tokens_in":   data.tokensIn,
			"tokens_out":  data.tokensOut,
		})
	}

	// Sort labels chronologically
	for i := 0; i < len(labels)-1; i++ {
		for j := i + 1; j < len(labels); j++ {
			if labels[j] < labels[i] {
				labels[i], labels[j] = labels[j], labels[i]
				dataPoints[i], dataPoints[j] = dataPoints[j], dataPoints[i]
			}
		}
	}

	trends["labels"] = labels
	trends["data"] = dataPoints
	return trends, nil
}
