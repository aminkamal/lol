package cruncher

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strings"
	"time"

	"github.com/aminkamal/lol/pkg/logger"
	"github.com/aminkamal/lol/pkg/riot"
)

func WriteCleanedup(account *riot.GetByRiotIdResponse, matches []riot.GetMatchResponse) error {
	dirPath := strings.ToLower(account.GameName+"_"+account.TagLine) + "_cleanedup"

	// Create directory for the player's matches
	if err := os.MkdirAll(dirPath, 0755); err != nil {
		logger.Error("Failed to create directory: %v", err)
		return err
	}

	for _, match := range matches {
		cleanupData := make(map[string]any)

		cleanupData["matchId"] = match.Metadata.MatchID

		cleanupData["endOfGameResult"] = match.Info.EndOfGameResult
		cleanupData["gameCreation"] = match.Info.GameCreation
		cleanupData["gameDuration"] = match.Info.GameDuration
		cleanupData["gameEndTimestamp"] = time.UnixMilli(match.Info.GameEndTimestamp).Format("2006-01-02 15:04:05")
		cleanupData["gameId"] = match.Info.GameID
		cleanupData["gameMode"] = match.Info.GameMode
		cleanupData["gameName"] = match.Info.GameName
		cleanupData["gameStartTimestamp"] = time.UnixMilli(match.Info.GameStartTimestamp).Format("2006-01-02 15:04:05")
		cleanupData["gameType"] = match.Info.GameType
		cleanupData["gameVersion"] = match.Info.GameVersion
		cleanupData["mapId"] = match.Info.MapID
		cleanupData["platformId"] = match.Info.PlatformID

		for _, p := range match.Info.Participants {
			v := reflect.ValueOf(p)
			for i := 0; i < v.NumField(); i++ {
				n := v.Type().Field(i).Name
				// exclude riotidgamename and tagline because they're now glue partitions
				// exclude summonerName because it confuses the **** out of the SQL generator
				if n == "Missions" || n == "Perks" || n == "riotIdGameName" || n == "riotIdTagline" || n == "summonerName" {
					continue
				}
				if n == "Challenges" {
					vv := reflect.ValueOf(p.Challenges)
					cleanupData["challenge_"+vv.Type().Field(i).Name] = vv.Field(i).Interface()
					continue
				}
				cleanupData[n] = v.Field(i).Interface()
			}

			slug := match.Metadata.MatchID + "_" + p.RiotIDGameName + "_" + p.RiotIDTagline
			fileName := fmt.Sprintf("%s/match_%s.json", dirPath, slug)
			file, err := os.Create(fileName)
			if err != nil {
				logger.Error("Failed to create file: %v", err)
				return err
			}
			encoder := json.NewEncoder(file)
			encoder.SetIndent("", "  ")
			if err := encoder.Encode(cleanupData); err != nil {
				logger.Error("Failed to write JSON to file: %v", err)
				return err
			}
			file.Close()
		}
	}

	return nil
}
