package scraper

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/aminkamal/lol/pkg/logger"
	"github.com/aminkamal/lol/pkg/riot"
)

type Scraper struct {
	client *riot.Client
}

func New(client *riot.Client) *Scraper {
	return &Scraper{
		client: client,
	}
}

func (s *Scraper) Scrape(
	ctx context.Context,
	gameName string,
	tagLine string,
	region string,
	from time.Time,
	to time.Time,
) (*riot.GetByRiotIdResponse, error) {
	account, err := s.client.GetPUUIDByRiotID(region, gameName, tagLine)
	if err != nil {
		return nil, err
	}

	playerFullname := strings.ToLower(account.GameName + "#" + account.TagLine)
	playerSafename := strings.ToLower(account.GameName + "_" + account.TagLine)
	logger.Debug("Found account: %s (PUUID: %s)", playerFullname, account.Puuid)

	// Create directory for the player's matches
	if err := os.MkdirAll(playerSafename, 0755); err != nil {
		log.Fatalf("Failed to create directory: %v", err)
	}

	start := 0

	for {
		shouldBreak := false
		select {
		case <-ctx.Done():
			logger.Debug("Context cancelled while getting matches for %s", playerFullname)
			shouldBreak = true
		default:
		}

		if shouldBreak {
			break
		}

		// Get matches for PUUID
		matches, err := s.client.GetMatchesForPUUID(region, account.Puuid, from.Unix(), to.Unix(), start)
		if err != nil {
			return nil, err
		}

		logger.Debug("Retrieved %d match IDs for %s", len(*matches), playerFullname)

		if len(*matches) == 0 {
			logger.Debug("No more matches for user %s", playerFullname)
			break
		}

		for _, matchId := range *matches {
			fileName := fmt.Sprintf("%s/match_%s.json", playerSafename, matchId)
			if fileExists(fileName) {
				continue
			}

			matchDetails, err := s.client.GetMatchById(region, matchId)
			if err != nil {
				return nil, err
			}

			// logger.Debug("Match details: %+v", matchDetails)
			// logger.Debug("Match end time %+v", matchDetails.Info.GameEndTimestamp)

			// Write match details to JSON file
			file, err := os.Create(fileName)
			if err != nil {
				log.Fatalf("Failed to create file: %v", err)
			}
			defer file.Close()

			encoder := json.NewEncoder(file)
			encoder.SetIndent("", "  ")
			if err := encoder.Encode(matchDetails); err != nil {
				logger.Error("Failed to write JSON to file: %v", err)
				return nil, err
			}

			logger.Debug("[%s] Match details written\n", time.UnixMilli(matchDetails.Info.GameEndTimestamp).String())
		}

		start += len(*matches)
	}

	return account, nil
}

func fileExists(filename string) bool {
	_, err := os.Stat(filename)
	if err == nil {
		return true // file exists
	}
	if errors.Is(err, os.ErrNotExist) {
		return false // file does not exist
	}
	// Some other error occurred (permission issues, etc.)
	return false
}
