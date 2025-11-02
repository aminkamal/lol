package riot

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/aminkamal/lol/pkg/logger"
)

// RateLimiter implements a sliding window rate limiter
type RateLimiter struct {
	mu         sync.Mutex
	timestamps []time.Time
	maxReqs    int
	window     time.Duration
}

// newRateLimiter creates a new rate limiter
func newRateLimiter(maxReqs int, window time.Duration) *RateLimiter {
	return &RateLimiter{
		timestamps: make([]time.Time, 0),
		maxReqs:    maxReqs,
		window:     window,
	}
}

// Wait blocks until a request can be made within the rate limit
func (rl *RateLimiter) Wait() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-rl.window)

	// Remove timestamps outside the window
	i := 0
	for i < len(rl.timestamps) && rl.timestamps[i].Before(cutoff) {
		i++
	}
	rl.timestamps = rl.timestamps[i:]

	// If we've hit the limit, wait until the oldest request expires
	if len(rl.timestamps) >= rl.maxReqs {
		oldestReq := rl.timestamps[0]
		sleepDuration := rl.window - now.Sub(oldestReq)
		if sleepDuration > 0 {
			logger.Warn("Rate limit reached. Waiting %v...\n", sleepDuration.Round(time.Second))
			rl.mu.Unlock()
			time.Sleep(sleepDuration)
			rl.mu.Lock()

			// Clean up again after sleeping
			now = time.Now()
			cutoff = now.Add(-rl.window)
			i = 0
			for i < len(rl.timestamps) && rl.timestamps[i].Before(cutoff) {
				i++
			}
			rl.timestamps = rl.timestamps[i:]
		}
	}

	// Record this request
	rl.timestamps = append(rl.timestamps, now)
}

// Client handles HTTP requests to the Riot API
type Client struct {
	client      *http.Client
	apiKey      string
	rateLimiter *RateLimiter
}

// NewClient creates a new Riot API client
func NewClient(apiKey string) *Client {
	return &Client{
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
		apiKey:      apiKey,
		rateLimiter: newRateLimiter(100, 2*time.Minute+10*time.Second),
	}
}

// doRequest performs an HTTP GET request and unmarshals the response
func (rc *Client) doRequest(url string, target interface{}) error {
	// Wait for rate limiter before making request
	rc.rateLimiter.Wait()

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Add("X-Riot-Token", rc.apiKey)

	resp, err := rc.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	// Output the response status and body (for debugging)
	// fmt.Println("Response Status:", resp.Status)
	// fmt.Println("Response Body:", string(body))

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API returned non-200 status: %s", resp.Status)
	}

	if err := json.Unmarshal(body, target); err != nil {
		return fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return nil
}

// GetMatchesForPUUID retrieves match IDs for a given player
func (rc *Client) GetMatchesForPUUID(region string, puuid string, startTs int64, endTs int64, start int) (*getMatchIdsByPUUIDResponse, error) {
	url := fmt.Sprintf("https://%s.api.riotgames.com/lol/match/v5/matches/by-puuid/%s/ids?type=ranked&startTime=%d&endTime=%d&count=100&start=%d",
		region, puuid, startTs, endTs, start)

	var matchIds getMatchIdsByPUUIDResponse
	if err := rc.doRequest(url, &matchIds); err != nil {
		return nil, err
	}

	return &matchIds, nil
}

// GetMatchById retrieves detailed match information by match ID
func (rc *Client) GetMatchById(region string, matchId string) (*GetMatchResponse, error) {
	url := fmt.Sprintf("https://%s.api.riotgames.com/lol/match/v5/matches/%s", region, matchId)

	var match GetMatchResponse
	if err := rc.doRequest(url, &match); err != nil {
		return nil, err
	}

	return &match, nil
}

// GetPUUIDByRiotID retrieves a player's PUUID by their Riot ID
// There are three routing values for account-v1; americas, asia, and europe.
// You can query for any account in any region. We recommend using the nearest cluster.
func (rc *Client) GetPUUIDByRiotID(region string, gameName string, tagLine string) (*GetByRiotIdResponse, error) {
	url := fmt.Sprintf("https://%s.api.riotgames.com/riot/account/v1/accounts/by-riot-id/%s/%s",
		region, gameName, tagLine)

	var account GetByRiotIdResponse
	if err := rc.doRequest(url, &account); err != nil {
		return nil, err
	}

	return &account, nil
}
