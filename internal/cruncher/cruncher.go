package cruncher

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/aminkamal/lol/pkg/riot"
)

// YearEndSummary contains all calculated statistics
type YearEndSummary struct {
	BasicStats            BasicStats               `json:"basicStats"`
	PerformanceMetrics    PerformanceMetrics       `json:"performanceMetrics"`
	ChampionStats         map[string]*ChampionStat `json:"championStats"`
	RoleDistribution      map[string]int           `json:"roleDistribution"`
	AchievementHighlights Achievements             `json:"achievements"`
	PlayPatterns          PlayPatterns             `json:"playPatterns"`
	ProgressTimeline      []MonthlyStats           `json:"progressTimeline"`
	SignatureMoments      SignatureMoments         `json:"signatureMoments"`
	TeamPlayerScore       TeamPlayerMetrics        `json:"teamPlayerScore"`
	FunStats              FunStats                 `json:"fun_statistics"`
}

type BasicStats struct {
	TotalGames         int     `json:"totalGames"`
	TotalWins          int     `json:"totalWins"`
	TotalLosses        int     `json:"totalLosses"`
	WinRate            float64 `json:"winRate"`
	TotalKills         int     `json:"totalKills"`
	TotalDeaths        int     `json:"totalDeaths"`
	TotalAssists       int     `json:"totalAssists"`
	OverallKDA         float64 `json:"overallKDA"`
	TotalDamageDealt   int64   `json:"totalDamageDealt"`
	TotalGoldEarned    int64   `json:"totalGoldEarned"`
	TotalTimePlayedMin int     `json:"totalTimePlayedMinutes"`
}

type PerformanceMetrics struct {
	AvgDamagePerMin      float64 `json:"avgDamagePerMinute"`
	AvgGoldPerMin        float64 `json:"avgGoldPerMinute"`
	AvgVisionScorePerMin float64 `json:"avgVisionScorePerMinute"`
	AvgKillParticipation float64 `json:"avgKillParticipation"`
	AvgTeamDamageShare   float64 `json:"avgTeamDamageShare"`
	BestKDA              float64 `json:"bestKDA"`
	BestKDAGame          string  `json:"bestKDAGameId"`
}

type ChampionStat struct {
	Name         string  `json:"name"`
	GamesPlayed  int     `json:"gamesPlayed"`
	Wins         int     `json:"wins"`
	Losses       int     `json:"losses"`
	WinRate      float64 `json:"winRate"`
	AvgKDA       float64 `json:"avgKDA"`
	TotalKills   int     `json:"totalKills"`
	TotalDeaths  int     `json:"totalDeaths"`
	TotalAssists int     `json:"totalAssists"`
}

type Achievements struct {
	FirstBloods       int `json:"firstBloods"`
	FirstTowers       int `json:"firstTowers"`
	PerfectGames      int `json:"perfectGames"`
	DoubleKills       int `json:"doubleKills"`
	TripleKills       int `json:"tripleKills"`
	QuadraKills       int `json:"quadraKills"`
	PentaKills        int `json:"pentaKills"`
	SoloKills         int `json:"totalSoloKills"`
	EpicMonsterSteals int `json:"epicMonsterSteals"`
	ComebackWins      int `json:"comebackWins"`
}

type PlayPatterns struct {
	GamesPerDay       map[string]int `json:"gamesPerDay"`
	PeakPlayDay       string         `json:"peakPlayDay"`
	PeakPlayCount     int            `json:"peakPlayCount"`
	AverageGameLength float64        `json:"avgGameLengthMinutes"`
	ShortestGame      float64        `json:"shortestGameMinutes"`
	LongestGame       float64        `json:"longestGameMinutes"`
	PreferredRole     string         `json:"preferredRole"`
	RoleDiversity     float64        `json:"roleDiversityScore"`
}

type SignatureMoments struct {
	LongestKillingSpree   int    `json:"longestKillingSpree"`
	LongestKillStreakGame string `json:"longestKillStreakGameId"`
	MostDamageInGame      int    `json:"mostDamageInGame"`
	MostDamageGameID      string `json:"mostDamageGameId"`
	MostKillsInGame       int    `json:"mostKillsInGame"`
	MostKillsGameID       string `json:"mostKillsGameId"`
	BiggestComeback       int    `json:"biggestComebackDeficit"`
	ClutchPlays           int    `json:"totalClutchPlays"`
	ClosestToPenta        int    `json:"closestToPentaKills"`
}

type TeamPlayerMetrics struct {
	TotalWardsPlaced       int     `json:"totalWardsPlaced"`
	TotalWardsDestroyed    int     `json:"totalWardsDestroyed"`
	AvgVisionScore         float64 `json:"avgVisionScore"`
	TotalAssists           int     `json:"totalAssists"`
	AssistStreaks          int     `json:"assistStreaks"`
	ObjectiveParticipation float64 `json:"objectiveParticipationRate"`
	SavedAllies            int     `json:"savedAllies"`
}

type MonthlyStats struct {
	Month            string  `json:"month"`
	Year             int     `json:"year"`
	GamesPlayed      int     `json:"gamesPlayed"`
	WinRate          float64 `json:"winRate"`
	AvgKDA           float64 `json:"avgKDA"`
	AvgDamagePerMin  float64 `json:"avgDamagePerMinute"`
	FavoriteChampion string  `json:"favoriteChampion"`
}

// YearSummaryProcessor processes matches for a specific player
type YearSummaryProcessor struct {
	RiotIdGameName string
	RiotIdTagLine  string
	Matches        []riot.GetMatchResponse
	Summary        *YearEndSummary
}

// NewYearSummaryProcessor creates a new processor instance
func NewYearSummaryProcessor(riotIdGameName, riotIDTagline string) *YearSummaryProcessor {
	return &YearSummaryProcessor{
		RiotIdGameName: riotIdGameName,
		RiotIdTagLine:  riotIDTagline,
		Matches:        []riot.GetMatchResponse{},
		Summary: &YearEndSummary{
			ChampionStats:    make(map[string]*ChampionStat),
			RoleDistribution: make(map[string]int),
		},
	}
}

// ProcessMatches analyzes all matches and generates summary
func (p *YearSummaryProcessor) ProcessMatches() {
	p.calculateBasicStats()
	p.calculatePerformanceMetrics()
	p.calculateChampionStats()
	p.calculateRoleDistribution()
	p.calculateAchievements()
	p.calculatePlayPatterns()
	p.calculateSignatureMoments()
	p.calculateTeamPlayerScore()
	p.generateProgressTimeline()
	p.CalculateFunStats()
}

func (p *YearSummaryProcessor) calculateBasicStats() {
	stats := &p.Summary.BasicStats

	for _, match := range p.Matches {
		participant := p.findPlayerParticipant(match)
		if participant == nil {
			continue
		}

		stats.TotalGames++
		if participant.Win {
			stats.TotalWins++
		} else {
			stats.TotalLosses++
		}

		stats.TotalKills += participant.Kills
		stats.TotalDeaths += participant.Deaths
		stats.TotalAssists += participant.Assists
		stats.TotalDamageDealt += int64(participant.TotalDamageDealtToChampions)
		stats.TotalGoldEarned += int64(participant.GoldEarned)
		stats.TotalTimePlayedMin += participant.TimePlayed / 60
	}

	if stats.TotalGames > 0 {
		stats.WinRate = float64(stats.TotalWins) / float64(stats.TotalGames) * 100

		if stats.TotalDeaths > 0 {
			stats.OverallKDA = float64(stats.TotalKills+stats.TotalAssists) / float64(stats.TotalDeaths)
		} else {
			stats.OverallKDA = float64(stats.TotalKills + stats.TotalAssists)
		}
	}
}

func (p *YearSummaryProcessor) calculatePerformanceMetrics() {
	metrics := &p.Summary.PerformanceMetrics

	var totalDPM, totalGPM, totalVSPM, totalKP, totalDmgShare float64
	var count int
	bestKDA := 0.0
	bestKDAGame := ""

	for _, match := range p.Matches {
		participant := p.findPlayerParticipant(match)
		if participant == nil {
			continue
		}

		count++
		totalDPM += participant.Challenges.DamagePerMinute
		totalGPM += participant.Challenges.GoldPerMinute
		totalVSPM += participant.Challenges.VisionScorePerMinute
		totalKP += participant.Challenges.KillParticipation
		totalDmgShare += participant.Challenges.TeamDamagePercentage

		kda := participant.Challenges.Kda
		if kda > bestKDA {
			bestKDA = kda
			bestKDAGame = match.Metadata.MatchID
		}
	}

	if count > 0 {
		metrics.AvgDamagePerMin = totalDPM / float64(count)
		metrics.AvgGoldPerMin = totalGPM / float64(count)
		metrics.AvgVisionScorePerMin = totalVSPM / float64(count)
		metrics.AvgKillParticipation = totalKP / float64(count)
		metrics.AvgTeamDamageShare = totalDmgShare / float64(count)
		metrics.BestKDA = bestKDA
		metrics.BestKDAGame = bestKDAGame
	}
}

func (p *YearSummaryProcessor) calculateChampionStats() {
	for _, match := range p.Matches {
		participant := p.findPlayerParticipant(match)
		if participant == nil {
			continue
		}

		champName := participant.ChampionName
		if _, exists := p.Summary.ChampionStats[champName]; !exists {
			p.Summary.ChampionStats[champName] = &ChampionStat{
				Name: champName,
			}
		}

		stat := p.Summary.ChampionStats[champName]
		stat.GamesPlayed++
		stat.TotalKills += participant.Kills
		stat.TotalDeaths += participant.Deaths
		stat.TotalAssists += participant.Assists

		if participant.Win {
			stat.Wins++
		} else {
			stat.Losses++
		}
	}

	// Calculate win rates and KDA for each champion
	for _, stat := range p.Summary.ChampionStats {
		if stat.GamesPlayed > 0 {
			stat.WinRate = float64(stat.Wins) / float64(stat.GamesPlayed) * 100

			if stat.TotalDeaths > 0 {
				stat.AvgKDA = float64(stat.TotalKills+stat.TotalAssists) / float64(stat.TotalDeaths)
			} else {
				stat.AvgKDA = float64(stat.TotalKills + stat.TotalAssists)
			}
		}
	}
}

func (p *YearSummaryProcessor) calculateRoleDistribution() {
	for _, match := range p.Matches {
		participant := p.findPlayerParticipant(match)
		if participant == nil {
			continue
		}

		role := participant.TeamPosition
		if role == "" {
			role = participant.IndividualPosition
		}
		if role != "" {
			p.Summary.RoleDistribution[role]++
		}
	}
}

func (p *YearSummaryProcessor) calculateAchievements() {
	achievements := &p.Summary.AchievementHighlights

	for _, match := range p.Matches {
		participant := p.findPlayerParticipant(match)
		if participant == nil {
			continue
		}

		if participant.FirstBloodKill {
			achievements.FirstBloods++
		}
		if participant.FirstTowerKill {
			achievements.FirstTowers++
		}
		if participant.Challenges.PerfectGame > 0 {
			achievements.PerfectGames++
		}

		achievements.DoubleKills += participant.DoubleKills
		achievements.TripleKills += participant.TripleKills
		achievements.QuadraKills += participant.QuadraKills
		achievements.PentaKills += participant.PentaKills
		achievements.SoloKills += participant.Challenges.SoloKills
		achievements.EpicMonsterSteals += participant.Challenges.EpicMonsterSteals

		if participant.Win && participant.Challenges.MaxKillDeficit > 0 {
			achievements.ComebackWins++
		}
	}
}

func (p *YearSummaryProcessor) calculatePlayPatterns() {
	patterns := &p.Summary.PlayPatterns
	patterns.GamesPerDay = make(map[string]int)

	var totalGameLength float64
	shortestGame := math.MaxFloat64
	longestGame := 0.0

	for _, match := range p.Matches {
		// Track games per day
		date := time.Unix(match.Info.GameCreation/1000, 0).Format("2006-01-02")
		patterns.GamesPerDay[date]++

		// Track game lengths
		gameLength := float64(match.Info.GameDuration) / 60.0
		totalGameLength += gameLength

		if gameLength < shortestGame {
			shortestGame = gameLength
		}
		if gameLength > longestGame {
			longestGame = gameLength
		}
	}

	// Find peak play day
	maxGames := 0
	peakDay := ""
	for day, count := range patterns.GamesPerDay {
		if count > maxGames {
			maxGames = count
			peakDay = day
		}
	}

	patterns.PeakPlayDay = peakDay
	patterns.PeakPlayCount = maxGames

	if len(p.Matches) > 0 {
		patterns.AverageGameLength = totalGameLength / float64(len(p.Matches))
		patterns.ShortestGame = shortestGame
		patterns.LongestGame = longestGame
	}

	// Find preferred role
	maxRole := 0
	preferredRole := ""
	for role, count := range p.Summary.RoleDistribution {
		if count > maxRole {
			maxRole = count
			preferredRole = role
		}
	}
	patterns.PreferredRole = preferredRole

	// Calculate role diversity (Shannon entropy)
	patterns.RoleDiversity = p.calculateRoleDiversity()
}

func (p *YearSummaryProcessor) calculateRoleDiversity() float64 {
	total := 0
	for _, count := range p.Summary.RoleDistribution {
		total += count
	}

	if total == 0 {
		return 0
	}

	entropy := 0.0
	for _, count := range p.Summary.RoleDistribution {
		if count > 0 {
			prob := float64(count) / float64(total)
			entropy -= prob * math.Log2(prob)
		}
	}

	// Normalize to 0-100 scale
	maxEntropy := math.Log2(5) // 5 roles maximum
	return (entropy / maxEntropy) * 100
}

func (p *YearSummaryProcessor) calculateSignatureMoments() {
	moments := &p.Summary.SignatureMoments

	for _, match := range p.Matches {
		participant := p.findPlayerParticipant(match)
		if participant == nil {
			continue
		}

		// Track longest killing spree
		if participant.LargestKillingSpree > moments.LongestKillingSpree {
			moments.LongestKillingSpree = participant.LargestKillingSpree
			moments.LongestKillStreakGame = match.Metadata.MatchID
		}

		// Track most damage in a game
		if participant.TotalDamageDealtToChampions > moments.MostDamageInGame {
			moments.MostDamageInGame = participant.TotalDamageDealtToChampions
			moments.MostDamageGameID = match.Metadata.MatchID
		}

		// Track most kills in a game
		if participant.Kills > moments.MostKillsInGame {
			moments.MostKillsInGame = participant.Kills
			moments.MostKillsGameID = match.Metadata.MatchID
		}

		// Track biggest comeback
		if participant.Win && participant.Challenges.MaxKillDeficit > moments.BiggestComeback {
			moments.BiggestComeback = participant.Challenges.MaxKillDeficit
		}

		// Track clutch plays
		moments.ClutchPlays += participant.Challenges.OutnumberedKills

		// Track closest to penta
		if participant.QuadraKills > 0 && participant.PentaKills == 0 {
			moments.ClosestToPenta = 4
		} else if participant.LargestMultiKill > moments.ClosestToPenta && moments.ClosestToPenta < 5 {
			moments.ClosestToPenta = participant.LargestMultiKill
		}
	}
}

func (p *YearSummaryProcessor) calculateTeamPlayerScore() {
	metrics := &p.Summary.TeamPlayerScore

	var totalVisionScore float64
	count := 0

	for _, match := range p.Matches {
		participant := p.findPlayerParticipant(match)
		if participant == nil {
			continue
		}

		count++
		metrics.TotalWardsPlaced += participant.WardsPlaced
		metrics.TotalWardsDestroyed += participant.WardsKilled
		totalVisionScore += float64(participant.VisionScore)
		metrics.TotalAssists += participant.Assists

		// Count assist streaks (simplified - would need more data)
		if participant.Assists >= 12 {
			metrics.AssistStreaks++
		}

		// Track saves
		metrics.SavedAllies += participant.Challenges.SaveAllyFromDeath

		// Calculate objective participation (simplified)
		if participant.BaronKills > 0 || participant.DragonKills > 0 || participant.TurretTakedowns > 0 {
			metrics.ObjectiveParticipation++
		}
	}

	if count > 0 {
		metrics.AvgVisionScore = totalVisionScore / float64(count)
		metrics.ObjectiveParticipation = (metrics.ObjectiveParticipation / float64(count)) * 100
	}
}

func (p *YearSummaryProcessor) generateProgressTimeline() {
	// Group matches by month
	monthlyData := make(map[string]*MonthlyStats)
	monthlyMatches := make(map[string][]riot.GetMatchResponse)

	for _, match := range p.Matches {
		date := time.Unix(match.Info.GameCreation/1000, 0)
		monthKey := fmt.Sprintf("%d-%02d", date.Year(), date.Month())

		if _, exists := monthlyData[monthKey]; !exists {
			monthlyData[monthKey] = &MonthlyStats{
				Month: date.Month().String(),
				Year:  date.Year(),
			}
			monthlyMatches[monthKey] = []riot.GetMatchResponse{}
		}

		monthlyMatches[monthKey] = append(monthlyMatches[monthKey], match)
	}

	// Calculate stats for each month
	for monthKey, stats := range monthlyData {
		matches := monthlyMatches[monthKey]

		var wins, totalKills, totalDeaths, totalAssists int
		var totalDPM float64
		champCounts := make(map[string]int)

		for _, match := range matches {
			participant := p.findPlayerParticipant(match)
			if participant == nil {
				continue
			}

			stats.GamesPlayed++
			if participant.Win {
				wins++
			}

			totalKills += participant.Kills
			totalDeaths += participant.Deaths
			totalAssists += participant.Assists
			totalDPM += participant.Challenges.DamagePerMinute

			champCounts[participant.ChampionName]++
		}

		if stats.GamesPlayed > 0 {
			stats.WinRate = float64(wins) / float64(stats.GamesPlayed) * 100
			stats.AvgDamagePerMin = totalDPM / float64(stats.GamesPlayed)

			if totalDeaths > 0 {
				stats.AvgKDA = float64(totalKills+totalAssists) / float64(totalDeaths)
			} else {
				stats.AvgKDA = float64(totalKills + totalAssists)
			}

			// Find favorite champion for the month
			maxCount := 0
			for champ, count := range champCounts {
				if count > maxCount {
					maxCount = count
					stats.FavoriteChampion = champ
				}
			}
		}
	}

	// Convert to sorted slice
	var timeline []MonthlyStats
	for _, stats := range monthlyData {
		timeline = append(timeline, *stats)
	}

	sort.Slice(timeline, func(i, j int) bool {
		if timeline[i].Year != timeline[j].Year {
			return timeline[i].Year < timeline[j].Year
		}
		return timeline[i].Month < timeline[j].Month
	})

	p.Summary.ProgressTimeline = timeline
}

func (p *YearSummaryProcessor) findPlayerParticipant(match riot.GetMatchResponse) *riot.Participant {
	for _, participant := range match.Info.Participants {
		// NA01 breaks this, Doublelift#NA01 is Doublelift#NA1 but Doublelift#NA1 is a different user
		/*participant.RiotIDTagline == p.RiotIdTagLine*/
		if participant.RiotIDGameName == p.RiotIdGameName {
			return &participant
		}
	}
	return nil
}

// GenerateReport creates a formatted report of the year-end summary
func (p *YearSummaryProcessor) GenerateReport() string {
	s := p.Summary

	report := fmt.Sprintf(`
=== YOUR YEAR IN THE RIFT ===

📊 BASIC STATS
• Total Games: %d (W: %d | L: %d)
• Win Rate: %.1f%%
• Overall KDA: %.2f
• Total Kills: %d | Deaths: %d | Assists: %d
• Time Played: %d hours

🎯 PERFORMANCE METRICS
• Average Damage/Min: %.1f
• Average Gold/Min: %.1f
• Average Vision Score/Min: %.2f
• Best KDA: %.2f (Game: %s)

🏆 ACHIEVEMENTS
• First Bloods: %d
• Perfect Games: %d
• Pentakills: %d | Quadras: %d | Triples: %d
• Solo Kills: %d
• Comeback Wins: %d

🎮 SIGNATURE MOMENTS
• Longest Killing Spree: %d
• Most Damage in Game: %d
• Most Kills in Game: %d
• Biggest Comeback: %d kill deficit

🤝 TEAM PLAYER SCORE
• Total Wards Placed: %d
• Average Vision Score: %.1f
• Total Assists: %d
• Objective Participation: %.1f%%

`,
		s.BasicStats.TotalGames, s.BasicStats.TotalWins, s.BasicStats.TotalLosses,
		s.BasicStats.WinRate, s.BasicStats.OverallKDA,
		s.BasicStats.TotalKills, s.BasicStats.TotalDeaths, s.BasicStats.TotalAssists,
		s.BasicStats.TotalTimePlayedMin/60,
		s.PerformanceMetrics.AvgDamagePerMin,
		s.PerformanceMetrics.AvgGoldPerMin,
		s.PerformanceMetrics.AvgVisionScorePerMin,
		s.PerformanceMetrics.BestKDA, s.PerformanceMetrics.BestKDAGame,
		s.AchievementHighlights.FirstBloods,
		s.AchievementHighlights.PerfectGames,
		s.AchievementHighlights.PentaKills,
		s.AchievementHighlights.QuadraKills,
		s.AchievementHighlights.TripleKills,
		s.AchievementHighlights.SoloKills,
		s.AchievementHighlights.ComebackWins,
		s.SignatureMoments.LongestKillingSpree,
		s.SignatureMoments.MostDamageInGame,
		s.SignatureMoments.MostKillsInGame,
		s.SignatureMoments.BiggestComeback,
		s.TeamPlayerScore.TotalWardsPlaced,
		s.TeamPlayerScore.AvgVisionScore,
		s.TeamPlayerScore.TotalAssists,
		s.TeamPlayerScore.ObjectiveParticipation,
	)

	// Add top champions
	report += "🦸 TOP CHAMPIONS\n"
	type champEntry struct {
		name string
		stat *ChampionStat
	}

	var champions []champEntry
	for name, stat := range s.ChampionStats {
		champions = append(champions, champEntry{name, stat})
	}

	sort.Slice(champions, func(i, j int) bool {
		return champions[i].stat.GamesPlayed > champions[j].stat.GamesPlayed
	})

	for i, entry := range champions {
		if i >= 3 {
			break
		}
		report += fmt.Sprintf("• %s: %d games (%.1f%% WR, %.2f KDA)\n",
			entry.name, entry.stat.GamesPlayed, entry.stat.WinRate, entry.stat.AvgKDA)
	}

	// Add role distribution
	report += "\n📍 ROLE DISTRIBUTION\n"
	for role, count := range s.RoleDistribution {
		percentage := float64(count) / float64(s.BasicStats.TotalGames) * 100
		report += fmt.Sprintf("• %s: %d games (%.1f%%)\n", role, count, percentage)
	}

	return report
}

// ExportToJSON exports the summary as JSON
func (p *YearSummaryProcessor) ExportToJSON() ([]byte, error) {
	return json.MarshalIndent(p.Summary, "", "  ")
}

// GenerateVisualizationData creates data structures optimized for visualization
func (p *YearSummaryProcessor) GenerateVisualizationData() *VisualizationData {
	viz := &VisualizationData{
		HeatmapCalendar:  p.generateHeatmapData(),
		PerformanceRadar: p.generateRadarData(),
		ChampionWheel:    p.generateChampionWheel(),
		ProgressChart:    p.generateProgressChart(),
		RoleWheel:        p.generateRoleWheel(),
	}
	return viz
}

type VisualizationData struct {
	HeatmapCalendar  []HeatmapDay      `json:"heatmapCalendar"`
	PerformanceRadar RadarChart        `json:"performanceRadar"`
	ChampionWheel    []ChampionSegment `json:"championWheel"`
	ProgressChart    []ProgressPoint   `json:"progressChart"`
	RoleWheel        []RoleSegment     `json:"roleWheel"`
}

type HeatmapDay struct {
	Date      string  `json:"date"`
	GameCount int     `json:"gameCount"`
	WinRate   float64 `json:"winRate"`
	AvgKDA    float64 `json:"avgKDA"`
	Intensity float64 `json:"intensity"` // 0-1 scale for color
}

type RadarChart struct {
	Labels []string  `json:"labels"`
	Values []float64 `json:"values"` // 0-100 scale
}

type ChampionSegment struct {
	Champion string  `json:"champion"`
	Games    int     `json:"games"`
	WinRate  float64 `json:"winRate"`
	Angle    float64 `json:"angle"` // For pie chart
	Color    string  `json:"color"`
}

type ProgressPoint struct {
	Date            string  `json:"date"`
	CumulativeGames int     `json:"cumulativeGames"`
	CumulativeWins  int     `json:"cumulativeWins"`
	RollingWinRate  float64 `json:"rollingWinRate"` // Last 20 games
	RollingKDA      float64 `json:"rollingKDA"`
}

type RoleSegment struct {
	Role       string  `json:"role"`
	Count      int     `json:"count"`
	Percentage float64 `json:"percentage"`
	Color      string  `json:"color"`
}

func (p *YearSummaryProcessor) generateHeatmapData() []HeatmapDay {
	dailyStats := make(map[string]*HeatmapDay)

	for _, match := range p.Matches {
		participant := p.findPlayerParticipant(match)
		if participant == nil {
			continue
		}

		date := time.Unix(match.Info.GameCreation/1000, 0).Format("2006-01-02")

		if _, exists := dailyStats[date]; !exists {
			dailyStats[date] = &HeatmapDay{
				Date: date,
			}
		}

		day := dailyStats[date]
		day.GameCount++

		if participant.Win {
			day.WinRate = (day.WinRate*float64(day.GameCount-1) + 100) / float64(day.GameCount)
		} else {
			day.WinRate = day.WinRate * float64(day.GameCount-1) / float64(day.GameCount)
		}

		kda := participant.Challenges.Kda
		day.AvgKDA = (day.AvgKDA*float64(day.GameCount-1) + kda) / float64(day.GameCount)
	}

	// Find max game count for intensity calculation
	maxGames := 0
	for _, day := range dailyStats {
		if day.GameCount > maxGames {
			maxGames = day.GameCount
		}
	}

	// Calculate intensity and convert to slice
	var heatmapData []HeatmapDay
	for _, day := range dailyStats {
		if maxGames > 0 {
			day.Intensity = float64(day.GameCount) / float64(maxGames)
		}
		heatmapData = append(heatmapData, *day)
	}

	// Sort by date
	sort.Slice(heatmapData, func(i, j int) bool {
		return heatmapData[i].Date < heatmapData[j].Date
	})

	return heatmapData
}

func (p *YearSummaryProcessor) generateRadarData() RadarChart {
	// Normalize all metrics to 0-100 scale
	radar := RadarChart{
		Labels: []string{
			"KDA",
			"Damage",
			"Gold",
			"Vision",
			"Objectives",
			"Team Play",
		},
		Values: make([]float64, 6),
	}

	// KDA (cap at 5 for 100%)
	radar.Values[0] = math.Min(p.Summary.BasicStats.OverallKDA*20, 100)

	// Damage per minute (cap at 800 for 100%)
	radar.Values[1] = math.Min(p.Summary.PerformanceMetrics.AvgDamagePerMin/8, 100)

	// Gold per minute (cap at 500 for 100%)
	radar.Values[2] = math.Min(p.Summary.PerformanceMetrics.AvgGoldPerMin/5, 100)

	// Vision score per minute (cap at 2 for 100%)
	radar.Values[3] = math.Min(p.Summary.PerformanceMetrics.AvgVisionScorePerMin*50, 100)

	// Objective participation
	radar.Values[4] = p.Summary.TeamPlayerScore.ObjectiveParticipation

	// Kill participation (team play indicator)
	radar.Values[5] = p.Summary.PerformanceMetrics.AvgKillParticipation * 100

	return radar
}

func (p *YearSummaryProcessor) generateChampionWheel() []ChampionSegment {
	var segments []ChampionSegment
	totalGames := p.Summary.BasicStats.TotalGames

	// Sort champions by games played
	type champEntry struct {
		name string
		stat *ChampionStat
	}

	var champions []champEntry
	for name, stat := range p.Summary.ChampionStats {
		champions = append(champions, champEntry{name, stat})
	}

	sort.Slice(champions, func(i, j int) bool {
		return champions[i].stat.GamesPlayed > champions[j].stat.GamesPlayed
	})

	// Take top 10 champions and group rest as "Others"
	currentAngle := 0.0
	colors := []string{
		"#FF6B6B", "#4ECDC4", "#45B7D1", "#FFA07A", "#98D8C8",
		"#F7DC6F", "#BB8FCE", "#85C1E2", "#F8B739", "#52C234",
	}

	for i, entry := range champions {
		if i >= 10 {
			break
		}

		angle := float64(entry.stat.GamesPlayed) / float64(totalGames) * 360
		segment := ChampionSegment{
			Champion: entry.name,
			Games:    entry.stat.GamesPlayed,
			WinRate:  entry.stat.WinRate,
			Angle:    angle,
			Color:    colors[i%len(colors)],
		}
		currentAngle += angle
		segments = append(segments, segment)
	}

	// Add "Others" segment if needed
	othersCount := 0
	for i := 10; i < len(champions); i++ {
		othersCount += champions[i].stat.GamesPlayed
	}

	if othersCount > 0 {
		angle := float64(othersCount) / float64(totalGames) * 360
		segments = append(segments, ChampionSegment{
			Champion: "Others",
			Games:    othersCount,
			WinRate:  0, // Would need to calculate
			Angle:    angle,
			Color:    "#CCCCCC",
		})
	}

	return segments
}

func (p *YearSummaryProcessor) generateProgressChart() []ProgressPoint {
	// Sort matches by date
	sortedMatches := make([]riot.GetMatchResponse, len(p.Matches))
	copy(sortedMatches, p.Matches)
	sort.Slice(sortedMatches, func(i, j int) bool {
		return sortedMatches[i].Info.GameCreation < sortedMatches[j].Info.GameCreation
	})

	var points []ProgressPoint
	cumulativeGames := 0
	cumulativeWins := 0

	// Rolling window for win rate and KDA (last 20 games)
	windowSize := 20
	rollingWindow := []riot.GetMatchResponse{}

	for _, match := range sortedMatches {
		participant := p.findPlayerParticipant(match)
		if participant == nil {
			continue
		}

		cumulativeGames++
		if participant.Win {
			cumulativeWins++
		}

		// Update rolling window
		rollingWindow = append(rollingWindow, match)
		if len(rollingWindow) > windowSize {
			rollingWindow = rollingWindow[1:]
		}

		// Calculate rolling stats
		rollingWins := 0
		rollingKills := 0
		rollingDeaths := 0
		rollingAssists := 0

		for _, m := range rollingWindow {
			p := p.findPlayerParticipant(m)
			if p != nil {
				if p.Win {
					rollingWins++
				}
				rollingKills += p.Kills
				rollingDeaths += p.Deaths
				rollingAssists += p.Assists
			}
		}

		rollingWinRate := float64(rollingWins) / float64(len(rollingWindow)) * 100
		rollingKDA := 0.0
		if rollingDeaths > 0 {
			rollingKDA = float64(rollingKills+rollingAssists) / float64(rollingDeaths)
		} else {
			rollingKDA = float64(rollingKills + rollingAssists)
		}

		date := time.Unix(match.Info.GameCreation/1000, 0).Format("2006-01-02")

		point := ProgressPoint{
			Date:            date,
			CumulativeGames: cumulativeGames,
			CumulativeWins:  cumulativeWins,
			RollingWinRate:  rollingWinRate,
			RollingKDA:      rollingKDA,
		}

		// Add point every 5 games to reduce data points
		if cumulativeGames%5 == 0 || cumulativeGames == len(sortedMatches) {
			points = append(points, point)
		}
	}

	return points
}

func (p *YearSummaryProcessor) generateRoleWheel() []RoleSegment {
	var segments []RoleSegment
	totalGames := p.Summary.BasicStats.TotalGames

	roleColors := map[string]string{
		"TOP":     "#E74C3C",
		"JUNGLE":  "#27AE60",
		"MIDDLE":  "#3498DB",
		"BOTTOM":  "#F39C12",
		"UTILITY": "#9B59B6",
		"SUPPORT": "#9B59B6",
	}

	for role, count := range p.Summary.RoleDistribution {
		percentage := float64(count) / float64(totalGames) * 100
		color := roleColors[role]
		if color == "" {
			color = "#95A5A6"
		}

		segments = append(segments, RoleSegment{
			Role:       role,
			Count:      count,
			Percentage: percentage,
			Color:      color,
		})
	}

	// Sort by count
	sort.Slice(segments, func(i, j int) bool {
		return segments[i].Count > segments[j].Count
	})

	return segments
}

// FunStats generates fun/quirky statistics
type FunStats struct {
	CoffeeBreakWarrior int     `json:"coffeeBreakGames"`     // Games under 20 min won
	MarathonMaster     float64 `json:"longestGame"`          // Longest game in minutes
	NightOwl           int     `json:"gamesAfterMidnight"`   // Games played after midnight
	EarlyBird          int     `json:"gamesBeforeNoon"`      // Games played before noon
	WardingMachine     string  `json:"wardingComparison"`    // "You placed X wards - that's one every Y minutes!"
	DamageDealer       string  `json:"damageComparison"`     // Fun comparison for total damage
	GoldHoarder        string  `json:"goldComparison"`       // Fun comparison for total gold
	DeathDefier        int     `json:"closeCalls"`           // Times survived with <10% HP
	FlashMaster        int     `json:"successfulFlashPlays"` // Kills/escapes after flash
}

func (p *YearSummaryProcessor) CalculateFunStats() {
	stats := FunStats{}

	for _, match := range p.Matches {
		participant := p.findPlayerParticipant(match)
		if participant == nil {
			continue
		}

		gameLength := float64(match.Info.GameDuration) / 60.0

		// Coffee Break Warrior
		if gameLength < 20 && participant.Win {
			stats.CoffeeBreakWarrior++
		}

		// Marathon Master
		if gameLength > stats.MarathonMaster {
			stats.MarathonMaster = gameLength
		}

		// Time-based stats
		gameTime := time.Unix(match.Info.GameStartTimestamp/1000, 0)
		hour := gameTime.Hour()

		if hour >= 0 && hour < 6 {
			stats.NightOwl++
		}
		if hour >= 6 && hour < 12 {
			stats.EarlyBird++
		}
	}

	// Fun comparisons
	totalWards := p.Summary.TeamPlayerScore.TotalWardsPlaced
	totalMinutes := p.Summary.BasicStats.TotalTimePlayedMin
	if totalMinutes > 0 {
		wardsPerHour := float64(totalWards) / (float64(totalMinutes) / 60)
		stats.WardingMachine = fmt.Sprintf("You placed %d wards - that's %.1f per hour of gameplay!",
			totalWards, wardsPerHour)
	}

	totalDamage := p.Summary.BasicStats.TotalDamageDealt
	stats.DamageDealer = fmt.Sprintf("You dealt %s total damage - that's like destroying %d turrets!",
		formatLargeNumber(totalDamage), totalDamage/5000)

	totalGold := p.Summary.BasicStats.TotalGoldEarned
	stats.GoldHoarder = fmt.Sprintf("You earned %s gold - enough to buy %d Infinity Edges!",
		formatLargeNumber(totalGold), totalGold/3400)

	p.Summary.FunStats = stats
}

func formatLargeNumber(n int64) string {
	if n >= 1000000 {
		return fmt.Sprintf("%.1fM", float64(n)/1000000)
	} else if n >= 1000 {
		return fmt.Sprintf("%.1fK", float64(n)/1000)
	}
	return fmt.Sprintf("%d", n)
}

func Crunch(account *riot.GetByRiotIdResponse, matches []riot.GetMatchResponse) []byte {
	// Create processor for a specific player
	processor := NewYearSummaryProcessor(account.GameName, account.TagLine)

	// Add matches (in real scenario, you'd load all matches for the year)
	// processor.Matches = append(processor.Matches, *match)
	processor.Matches = matches

	// Process all matches
	processor.ProcessMatches()

	// Generate report
	report := processor.GenerateReport()
	//fmt.Println(report)

	// Export to JSON
	/*jsonData, err := processor.ExportToJSON()
	if err != nil {
		logger.Error("Error exporting to JSON: %v\n", err)
		return nil
	}*/
	//fmt.Printf("Summary (JSON):\n%s\n", jsonData)

	// Generate visualization data
	//vizData := processor.GenerateVisualizationData()
	//vizJSON, _ := json.MarshalIndent(vizData, "", "  ")
	//fmt.Printf("Visualization Data:\n%s\n", vizJSON)

	return []byte(report)
	//return jsonData
}
