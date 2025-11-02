package cruncher

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/aminkamal/lol/pkg/logger"
	"github.com/aminkamal/lol/pkg/riot"
)

type MetadataJSON struct {
	MetadataAttributes struct {
		MatchID        string `json:"match_id"`
		MatchMonth     string `json:"match_month"`
		MatchResult    string `json:"match_result"`
		RiotIdGameName string `json:"riotidgamename"`
		RiotIdTagLine  string `json:"riotidtagline"`
	} `json:"metadataAttributes"`
}

func formatDuration(seconds int) string {
	mins := seconds / 60
	secs := seconds % 60
	return fmt.Sprintf("%d:%02d", mins, secs)
}

func formatTimestamp(timestamp int64) string {
	t := time.Unix(timestamp/1000, 0)
	return t.Format("Jan 2, 2006 15:04 MST")
}

func formatTimestampMonth(timestamp int64) string {
	t := time.Unix(timestamp/1000, 0)
	return t.Format("Jan")
}

func generateMarkdown(targetPlayer riot.Participant, data riot.GetMatchResponse) string {
	info := data.Info
	md := "# League of Legends Match Report\n\n"

	// Match Overview
	md += "## Match Overview\n\n"
	md += fmt.Sprintf("- Match ID: `%s`\n", data.Metadata.MatchID)
	md += fmt.Sprintf("- Game Mode: %s Ranked Solo/Duo\n", info.GameMode)
	md += fmt.Sprintf("- Patch: %s\n", info.GameVersion)
	md += fmt.Sprintf("- Duration: %s\n", formatDuration(info.GameDuration))
	md += fmt.Sprintf("- Started: %s\n", formatTimestamp(info.GameCreation))

	// Calculate game-wide statistics
	var totalKills, totalGold, totalDamage int
	var blueTeamKills, redTeamKills int
	didGmeEndedInSurrender := false
	for _, p := range info.Participants {
		totalKills += p.Kills
		totalGold += p.GoldEarned
		totalDamage += p.TotalDamageDealtToChampions
		if p.TeamID == 100 {
			blueTeamKills += p.Kills
		} else {
			redTeamKills += p.Kills
		}

		if p.GameEndedInSurrender {
			didGmeEndedInSurrender = true
		}
	}

	if didGmeEndedInSurrender {
		md += "- Result: Ended in surrender\n"
	} else {
		md += "- Result: Match completed\n"
	}
	md += "\n"

	md += "## Match Summary\n\n"
	md += fmt.Sprintf("- Total Kills: %d\n", totalKills)
	md += fmt.Sprintf("- Average Game Gold: %s\n", formatNumber(totalGold/len(info.Participants)))
	md += fmt.Sprintf("- Total Champion Damage: %s\n", formatNumber(totalDamage))
	md += fmt.Sprintf("- Kill Score: Blue %d - %d Red\n", blueTeamKills, redTeamKills)
	md += "- Players in match:\n"
	for _, p := range info.Participants {
		md += fmt.Sprintf("  - %s\n", p.RiotIDGameName)
	}

	// Calculate bloodiness
	bloodiness := "Low"
	killsPerMin := float64(totalKills) / (float64(info.GameDuration) / 60)
	if killsPerMin > 1.5 {
		bloodiness = "Very High (Fiesta)"
	} else if killsPerMin > 1.0 {
		bloodiness = "High"
	} else if killsPerMin > 0.7 {
		bloodiness = "Medium"
	}
	md += fmt.Sprintf("- Game Pace: %.1f kills/min (%s bloodiness)\n", killsPerMin, bloodiness)
	md += "\n"

	// Team Results
	md += "## Team Results\n\n"
	for _, team := range info.Teams {
		teamName := "Blue Team"
		if team.TeamID == 200 {
			teamName = "Red Team"
		}

		result := "Loss"
		if team.Win {
			result = "Win"
		}

		md += fmt.Sprintf("### %s - %s\n\n", teamName, result)

		// Team stats
		teamKills := blueTeamKills
		if team.TeamID == 200 {
			teamKills = redTeamKills
		}

		md += "Objectives Secured:\n"
		md += fmt.Sprintf("- Champion Kills: %d\n", teamKills)
		md += fmt.Sprintf("- Towers: %d/11\n", team.Objectives.Tower.Kills)
		md += fmt.Sprintf("- Dragons: %d\n", team.Objectives.Dragon.Kills)
		md += fmt.Sprintf("- Barons: %d\n", team.Objectives.Baron.Kills)

		// Dragon soul check
		if team.Objectives.Dragon.Kills >= 4 {
			md += "  - DRAGON SOUL OBTAINED\n"
		}

		md += "\n"
	}

	// MVP/ACE analysis
	md += "## MVP & Performance Highlights\n\n"

	var mvp, topDamage, topGold, topCS, topVision *riot.Participant
	var worstPlayer *riot.Participant
	maxScore := -1000.0
	minScore := 1000.0

	for i := range info.Participants {
		p := &info.Participants[i]

		// MVP score calculation
		kda := 0.0
		if p.Deaths > 0 {
			kda = float64(p.Kills+p.Assists) / float64(p.Deaths)
		} else {
			kda = float64(p.Kills + p.Assists)
		}
		score := kda*2 + p.Challenges.KillParticipation*10 +
			float64(p.VisionScore)/10 + float64(p.TotalDamageDealtToChampions)/10000

		if p.Win {
			score += 5
		}

		if score > maxScore {
			maxScore = score
			mvp = p
		}
		if score < minScore {
			minScore = score
			worstPlayer = p
		}

		// Track leaders
		if topDamage == nil || p.TotalDamageDealtToChampions > topDamage.TotalDamageDealtToChampions {
			topDamage = p
		}
		if topGold == nil || p.GoldEarned > topGold.GoldEarned {
			topGold = p
		}
		if topCS == nil || (p.TotalMinionsKilled+p.NeutralMinionsKilled) > (topCS.TotalMinionsKilled+topCS.NeutralMinionsKilled) {
			topCS = p
		}
		if topVision == nil || p.VisionScore > topVision.VisionScore {
			topVision = p
		}
	}

	if mvp != nil {
		mvpKDA := "Perfect"
		if mvp.Deaths > 0 {
			mvpKDA = fmt.Sprintf("%.2f", float64(mvp.Kills+mvp.Assists)/float64(mvp.Deaths))
		}
		md += fmt.Sprintf("### MVP: %s (%s)\n", mvp.RiotIDGameName, mvp.ChampionName)
		md += fmt.Sprintf("- %d/%d/%d (KDA: %s) | %.0f%% Kill Participation\n",
			mvp.Kills, mvp.Deaths, mvp.Assists, mvpKDA, mvp.Challenges.KillParticipation*100)
		md += fmt.Sprintf("- Damage: %s | Gold: %s\n\n",
			formatNumber(mvp.TotalDamageDealtToChampions), formatNumber(mvp.GoldEarned))
	}

	md += "### Category Leaders\n\n"
	if topDamage != nil {
		md += fmt.Sprintf("- Damage King: %s (%s) - %s to champions\n",
			topDamage.RiotIDGameName, topDamage.ChampionName, formatNumber(topDamage.TotalDamageDealtToChampions))
	}
	if topGold != nil {
		md += fmt.Sprintf("- Gold Leader: %s (%s) - %s earned\n",
			topGold.RiotIDGameName, topGold.ChampionName, formatNumber(topGold.GoldEarned))
	}
	if topCS != nil {
		totalCS := topCS.TotalMinionsKilled + topCS.NeutralMinionsKilled
		md += fmt.Sprintf("- CS Leader: %s (%s) - %d CS (%.1f/min)\n",
			topCS.RiotIDGameName, topCS.ChampionName, totalCS,
			float64(totalCS)/float64(info.GameDuration)*60)
	}
	if topVision != nil {
		md += fmt.Sprintf("- Vision MVP: %s (%s) - %d vision score\n",
			topVision.RiotIDGameName, topVision.ChampionName, topVision.VisionScore)
	}

	if worstPlayer != nil && !worstPlayer.Win {
		md += fmt.Sprintf("\n Needs Improvement: %s (%s) - %d/%d/%d\n",
			worstPlayer.RiotIDGameName, worstPlayer.ChampionName,
			worstPlayer.Kills, worstPlayer.Deaths, worstPlayer.Assists)
	}
	md += "\n"

	// Player Performance
	// md += "## Detailed Player Performance\n\n"

	// Group by teams
	var blueTeam, redTeam []riot.Participant
	for _, p := range info.Participants {
		if p.TeamID == 100 {
			blueTeam = append(blueTeam, p)
		} else {
			redTeam = append(redTeam, p)
		}
	}

	//md += "### Blue Team\n\n"
	//md = generateTeamPlayers(md, blueTeam, info.GameDuration)

	//md += "### Red Team\n\n"
	//md = generateTeamPlayers(md, redTeam, info.GameDuration)

	md += fmt.Sprintf("### %s%s Individual Performance\n\n", targetPlayer.RiotIDGameName, targetPlayer.RiotIDTagline)
	md = generateTeamPlayers(md, []riot.Participant{targetPlayer}, info.GameDuration)

	// Add comparative analysis
	// md += "## Comparative Analysis\n\n"
	// md = generateComparativeAnalysis(md, info.Participants, info.GameDuration)

	// Add fun facts section
	md += "### Fun Facts & Trivia\n\n"
	md = generateFunFacts(md, info.Participants, info.GameDuration)

	return md
}

func generateTeamPlayers(md string, players []riot.Participant, gameDuration int) string {
	for _, p := range players {
		kda := "Perfect"
		kdaVal := 0.0
		if p.Deaths > 0 {
			kdaVal = float64(p.Kills+p.Assists) / float64(p.Deaths)
			kda = fmt.Sprintf("%.2f", kdaVal)
		} else {
			kdaVal = float64(p.Kills + p.Assists)
		}

		// Performance rating
		rating := "*"
		if kdaVal >= 5 {
			rating = "* (S+)"
		} else if kdaVal >= 3 {
			rating = " (S)"
		} else if kdaVal >= 2 {
			rating = "* (A)"
		} else if kdaVal >= 1 {
			rating = " (B)"
		} else {
			rating = "* (C)"
		}

		result := "Loss"
		if p.Win {
			result = "Win"
		}

		md += fmt.Sprintf("#### %s %s#%s - %s\n\n",
			result, p.RiotIDGameName, p.RiotIDTagline, p.ChampionName)

		md += fmt.Sprintf("%s | Position: %s | Level: %d\n\n", rating, p.TeamPosition, p.ChampLevel)

		// Key highlights
		if p.FirstBloodKill {
			md += "First Blood | "
		}
		if p.FirstBloodAssist {
			md += "First Blood Assist | "
		}
		if p.FirstTowerKill {
			md += "First Tower | "
		}
		if p.PentaKills > 0 {
			md += fmt.Sprintf("%d PENTAKILL(S) | ", p.PentaKills)
		}
		if p.QuadraKills > 0 {
			md += fmt.Sprintf("%d Quadra Kill(s) | ", p.QuadraKills)
		}
		if p.TripleKills > 0 {
			md += fmt.Sprintf("%d Triple Kill(s) | ", p.TripleKills)
		}
		if p.DoubleKills > 0 {
			md += fmt.Sprintf("%d Double Kill(s) | ", p.DoubleKills)
		}
		md += "\n\n"

		md += "#### Combat Stats\n\n"
		md += fmt.Sprintf("- KDA: %d/%d/%d (%s)\n", p.Kills, p.Deaths, p.Assists, kda)
		md += fmt.Sprintf("- Kill Participation: %.1f%%\n", p.Challenges.KillParticipation*100)
		if p.Challenges.SoloKills > 0 {
			md += fmt.Sprintf("- Solo Kills: %d\n", p.Challenges.SoloKills)
		}
		if p.LargestKillingSpree > 1 {
			md += fmt.Sprintf("- Largest Killing Spree: %d\n", p.LargestKillingSpree)
		}
		md += fmt.Sprintf("- Longest Time Alive: %s\n", formatDuration(p.LongestTimeSpentLiving))
		md += fmt.Sprintf("- Time Spent Dead: %s\n", formatDuration(p.TotalTimeSpentDead))

		md += "\n#### Damage Stats\n\n"
		md += fmt.Sprintf("- Total Damage to Champions: %s (%.1f DPM)\n",
			formatNumber(p.TotalDamageDealtToChampions),
			p.Challenges.DamagePerMinute)
		md += fmt.Sprintf("  - Physical: %s | Magic: %s | True: %s\n",
			formatNumber(p.PhysicalDamageDealtToChampions),
			formatNumber(p.MagicDamageDealtToChampions),
			formatNumber(p.TrueDamageDealtToChampions))
		md += fmt.Sprintf("- Team Damage Share: %.1f%%\n", p.Challenges.TeamDamagePercentage*100)
		if p.LargestCriticalStrike > 0 {
			md += fmt.Sprintf("- Largest Crit: %s\n", formatNumber(p.LargestCriticalStrike))
		}
		md += fmt.Sprintf("- Damage to Turrets: %s\n", formatNumber(p.DamageDealtToTurrets))
		md += fmt.Sprintf("- Damage to Objectives: %s\n", formatNumber(p.DamageDealtToObjectives))

		md += "\n#### Survivability\n\n"
		md += fmt.Sprintf("- Damage Taken: %s\n", formatNumber(p.TotalDamageTaken))
		md += fmt.Sprintf("  - Physical: %s | Magic: %s | True: %s\n",
			formatNumber(p.PhysicalDamageTaken),
			formatNumber(p.MagicDamageTaken),
			formatNumber(p.TrueDamageTaken))
		md += fmt.Sprintf("- Damage Mitigated: %s\n", formatNumber(p.DamageSelfMitigated))
		md += fmt.Sprintf("- Team Damage Taken Share: %.1f%%\n", p.Challenges.DamageTakenOnTeamPercentage*100)
		md += fmt.Sprintf("- Self Healing: %s\n", formatNumber(p.TotalHeal))
		if p.TotalHealsOnTeammates > 0 {
			md += fmt.Sprintf("- Healing to Teammates: %s\n", formatNumber(p.TotalHealsOnTeammates))
		}
		if p.TotalDamageShieldedOnTeammates > 0 {
			md += fmt.Sprintf("- Shielding to Teammates: %s\n", formatNumber(p.TotalDamageShieldedOnTeammates))
		}
		if p.TimeCCingOthers > 0 {
			md += fmt.Sprintf("- Crowd Control Time: %d seconds\n", p.TimeCCingOthers)
		}

		md += "\n#### Economy & Farm\n\n"
		md += fmt.Sprintf("- Gold Earned: %s (%.1f GPM)\n",
			formatNumber(p.GoldEarned),
			p.Challenges.GoldPerMinute)
		md += fmt.Sprintf("- Gold Spent: %s (%.1f%% efficiency)\n",
			formatNumber(p.GoldSpent),
			float64(p.GoldSpent)/float64(p.GoldEarned)*100)
		totalCS := p.TotalMinionsKilled + p.NeutralMinionsKilled
		md += fmt.Sprintf("- CS: %d (%.1f/min)\n",
			totalCS,
			float64(totalCS)/float64(gameDuration)*60)
		md += fmt.Sprintf("  - Lane Minions: %d | Jungle: %d\n", p.TotalMinionsKilled, p.NeutralMinionsKilled)
		if p.Challenges.MaxCsAdvantageOnLaneOpponent > 0 {
			md += fmt.Sprintf("- Max CS Advantage: +%d\n", p.Challenges.MaxCsAdvantageOnLaneOpponent)
		}
		if p.Challenges.MaxLevelLeadLaneOpponent > 0 {
			md += fmt.Sprintf("- Max Level Lead: +%d\n", p.Challenges.MaxLevelLeadLaneOpponent)
		}

		md += "\n#### Vision & Map Control\n\n"
		md += fmt.Sprintf("- Vision Score: %d (%.1f/min)\n",
			p.VisionScore,
			float64(p.VisionScore)/float64(gameDuration)*60)
		md += fmt.Sprintf("- Wards Placed: %d | Destroyed: %d\n", p.WardsPlaced, p.WardsKilled)
		if p.Challenges.ControlWardsPlaced > 0 {
			md += fmt.Sprintf("- Control Wards: %d\n", p.Challenges.ControlWardsPlaced)
		}

		md += "\n#### Objectives & Structures\n\n"
		if p.TurretKills > 0 {
			md += fmt.Sprintf("- Turrets Destroyed: %d", p.TurretKills)
			if p.Challenges.TurretPlatesTaken > 0 {
				md += fmt.Sprintf(" (+ %d plates)", p.Challenges.TurretPlatesTaken)
			}
			md += "\n"
		}
		if p.InhibitorKills > 0 {
			md += fmt.Sprintf("- Inhibitors Destroyed: %d\n", p.InhibitorKills)
		}
		if p.DragonKills > 0 {
			md += fmt.Sprintf("- Dragons Slain: %d\n", p.DragonKills)
		}
		if p.BaronKills > 0 {
			md += fmt.Sprintf("- Baron Nashor Kills: %d\n", p.BaronKills)
		}

		md += "\n#### Performance Metrics\n\n"
		if p.Challenges.SkillshotsDodged > 0 {
			md += fmt.Sprintf("- Skillshots Dodged: %d\n", p.Challenges.SkillshotsDodged)
		}

		// Calculate efficiency scores
		// dpm := p.Challenges.DamagePerMinute
		// gpm := p.Challenges.GoldPerMinute
		dmgPerGold := 0.0
		if p.GoldEarned > 0 {
			dmgPerGold = float64(p.TotalDamageDealtToChampions) / float64(p.GoldEarned)
		}

		md += fmt.Sprintf("- Gold Efficiency: %.2f damage per gold\n", dmgPerGold)

		deathTime := float64(p.TotalTimeSpentDead) / float64(gameDuration) * 100
		md += fmt.Sprintf("- Time Dead: %.1f%% of game\n", deathTime)

		aliveTime := float64(p.LongestTimeSpentLiving) / float64(gameDuration) * 100
		md += fmt.Sprintf("- Longest Life: %.1f%% of game\n", aliveTime)

		// Early game stats
		md += "\n#### Early Game Performance (0-10 min)\n\n"
		if p.Challenges.LaneMinionsFirst10Minutes > 0 {
			md += fmt.Sprintf("- CS @ 10: %d (%.1f CS/min)\n",
				p.Challenges.LaneMinionsFirst10Minutes,
				float64(p.Challenges.LaneMinionsFirst10Minutes)/10.0)

			// CS benchmarks
			csRating := ""
			if p.Challenges.LaneMinionsFirst10Minutes >= 90 {
				csRating = " (Challenger tier)"
			} else if p.Challenges.LaneMinionsFirst10Minutes >= 80 {
				csRating = " (Master tier)"
			} else if p.Challenges.LaneMinionsFirst10Minutes >= 70 {
				csRating = " (Diamond tier)"
			} else if p.Challenges.LaneMinionsFirst10Minutes < 50 {
				csRating = " (Needs work)"
			}
			if csRating != "" {
				md += fmt.Sprintf("  - Early CS Rating:%s\n", csRating)
			}
		}

		if p.Challenges.TakedownsFirstXMinutes > 0 {
			md += fmt.Sprintf("- Early Kills/Assists: %d\n", p.Challenges.TakedownsFirstXMinutes)
		}

		// Fun/impressive stats
		md += "\n#### Highlight Reel\n\n"
		funStats := false

		if p.Challenges.OutnumberedKills > 0 {
			md += fmt.Sprintf("- Clutch Player: Won %d outnumbered fight(s)!\n", p.Challenges.OutnumberedKills)
			funStats = true
		}

		if p.Challenges.KillsNearEnemyTurret > 0 {
			md += fmt.Sprintf("- Tower Diver: %d kill(s) near enemy turret\n", p.Challenges.KillsNearEnemyTurret)
			funStats = true
		}

		if p.Challenges.KillsUnderOwnTurret > 0 {
			md += fmt.Sprintf("- Home Defense: %d kill(s) under friendly turret\n", p.Challenges.KillsUnderOwnTurret)
			funStats = true
		}

		if p.Challenges.SurvivedThreeImmobilizesInFight > 0 {
			md += fmt.Sprintf("- CC Tank: Survived 3+ CC abilities %d time(s)\n", p.Challenges.SurvivedThreeImmobilizesInFight)
			funStats = true
		}

		if p.Challenges.TookLargeDamageSurvived > 0 {
			md += fmt.Sprintf("- Built Different: Survived massive damage burst(s) %d time(s)\n", p.Challenges.TookLargeDamageSurvived)
			funStats = true
		}

		// Death stats (funny but useful)
		if p.Deaths >= 10 {
			md += fmt.Sprintf("- Death Counter: %d deaths (%.1f per minute) - This is rough buddy\n",
				p.Deaths, float64(p.Deaths)/float64(gameDuration)*60)
			funStats = true
		}

		if deathTime > 30 {
			md += fmt.Sprintf("- Spectator Mode: Spent %.1f%% of game watching gray screen\n", deathTime)
			funStats = true
		}

		// Efficiency memes
		if float64(p.GoldSpent) < float64(p.GoldEarned)*0.7 {
			unspentGold := p.GoldEarned - p.GoldSpent
			md += fmt.Sprintf("- Gold Hoarder: Left %s unspent (%.0f%% efficiency) - BUY ITEMS!\n",
				formatNumber(unspentGold), float64(p.GoldSpent)/float64(p.GoldEarned)*100)
			funStats = true
		}

		// Damage share context
		if p.Challenges.TeamDamagePercentage < 0.10 && p.TeamPosition != "SUPPORT" {
			md += fmt.Sprintf("- Invisible Mode: Only %.1f%% team damage - Were you AFK?\n",
				p.Challenges.TeamDamagePercentage*100)
			funStats = true
		}

		if p.Challenges.TeamDamagePercentage > 0.35 {
			md += fmt.Sprintf("- Hard Carry: %.1f%% team damage - Your back must hurt!\n",
				p.Challenges.TeamDamagePercentage*100)
			funStats = true
		}

		// Vision memes
		if p.VisionScore < 15 && gameDuration > 1200 {
			md += "- 🙈 Blind Gameplay: Low vision score - Buy more wards!\n"
			funStats = true
		}

		// KDA memes
		if p.Kills == 0 && p.Assists == 0 && p.Deaths > 3 {
			md += fmt.Sprintf("- The Feeder Special: 0/0/%d KDA - Tough game\n", p.Deaths)
			funStats = true
		}

		if p.Deaths == 0 && (p.Kills > 5 || p.Assists > 8) {
			md += "- Deathless Victory: Perfect KDA - Clean gameplay!\n"
			funStats = true
		}

		if !funStats {
			md += "- Clean performance, no notable highlights\n"
		}

		// Items
		/*md += "\n#### Final Build\n\n"
		items := []int{p.Item0, p.Item1, p.Item2, p.Item3, p.Item4, p.Item5, p.Item6}
		md += "Items: "
		itemCount := 0
		for _, item := range items {
			if item != 0 {
				md += fmt.Sprintf("`%d` ", item)
				itemCount++
			}
		}
		if itemCount == 0 {
			md += "None"
		}
		md += "\n\n"
		*/

		// Add personalized recommendations
		// md += generatePlayerRecommendations(p, gameDuration)

		md += "\n---\n\n"
	}

	return md
}

func formatNumber(n int) string {
	if n >= 1000 {
		return fmt.Sprintf("%d,%03d", n/1000, n%1000)
	}
	return fmt.Sprintf("%d", n)
}

func generateComparativeAnalysis(md string, participants []riot.Participant, gameDuration int) string {
	md += "### Team Comparison\n\n"

	// Calculate team totals
	var blueStats, redStats struct {
		Kills, Deaths, Assists int
		Damage, Gold, CS       int
		Vision                 int
		Turrets, Dragons       int
	}

	for _, p := range participants {
		stats := &blueStats
		if p.TeamID == 200 {
			stats = &redStats
		}

		stats.Kills += p.Kills
		stats.Deaths += p.Deaths
		stats.Assists += p.Assists
		stats.Damage += p.TotalDamageDealtToChampions
		stats.Gold += p.GoldEarned
		stats.CS += p.TotalMinionsKilled + p.NeutralMinionsKilled
		stats.Vision += p.VisionScore
		stats.Turrets += p.TurretKills
		stats.Dragons += p.DragonKills
	}

	md += "| Metric | Blue Team | Red Team | Advantage |\n"
	md += "|--------|-----------|----------|----------|\n"

	// Kills
	advantage := "Even"
	if blueStats.Kills > redStats.Kills {
		advantage = fmt.Sprintf("🔵 +%d", blueStats.Kills-redStats.Kills)
	} else if redStats.Kills > blueStats.Kills {
		advantage = fmt.Sprintf("🔴 +%d", redStats.Kills-blueStats.Kills)
	}
	md += fmt.Sprintf("| Kills | %d | %d | %s |\n", blueStats.Kills, redStats.Kills, advantage)

	// Gold
	advantage = "Even"
	if blueStats.Gold > redStats.Gold {
		advantage = fmt.Sprintf("🔵 +%s", formatNumber(blueStats.Gold-redStats.Gold))
	} else if redStats.Gold > blueStats.Gold {
		advantage = fmt.Sprintf("🔴 +%s", formatNumber(redStats.Gold-blueStats.Gold))
	}
	md += fmt.Sprintf("| Total Gold | %s | %s | %s |\n",
		formatNumber(blueStats.Gold), formatNumber(redStats.Gold), advantage)

	// Damage
	advantage = "Even"
	if blueStats.Damage > redStats.Damage {
		advantage = fmt.Sprintf("🔵 +%s", formatNumber(blueStats.Damage-redStats.Damage))
	} else if redStats.Damage > blueStats.Damage {
		advantage = fmt.Sprintf("🔴 +%s", formatNumber(redStats.Damage-blueStats.Damage))
	}
	md += fmt.Sprintf("| Total Damage | %s | %s | %s |\n",
		formatNumber(blueStats.Damage), formatNumber(redStats.Damage), advantage)

	// CS
	advantage = "Even"
	if blueStats.CS > redStats.CS {
		advantage = fmt.Sprintf("🔵 +%d", blueStats.CS-redStats.CS)
	} else if redStats.CS > blueStats.CS {
		advantage = fmt.Sprintf("🔴 +%d", redStats.CS-blueStats.CS)
	}
	md += fmt.Sprintf("| Total CS | %d | %d | %s |\n", blueStats.CS, redStats.CS, advantage)

	// Vision
	advantage = "Even"
	if blueStats.Vision > redStats.Vision {
		advantage = fmt.Sprintf("🔵 +%d", blueStats.Vision-redStats.Vision)
	} else if redStats.Vision > blueStats.Vision {
		advantage = fmt.Sprintf("🔴 +%d", redStats.Vision-blueStats.Vision)
	}
	md += fmt.Sprintf("| Vision Score | %d | %d | %s |\n", blueStats.Vision, redStats.Vision, advantage)

	// Turrets
	advantage = "Even"
	if blueStats.Turrets > redStats.Turrets {
		advantage = fmt.Sprintf("🔵 +%d", blueStats.Turrets-redStats.Turrets)
	} else if redStats.Turrets > blueStats.Turrets {
		advantage = fmt.Sprintf("🔴 +%d", redStats.Turrets-blueStats.Turrets)
	}
	md += fmt.Sprintf("| Turrets | %d | %d | %s |\n", blueStats.Turrets, redStats.Turrets, advantage)

	md += "\n"

	// Add insights
	md += "### 🔍 Key Insights\n\n"

	goldDiff := blueStats.Gold - redStats.Gold
	if goldDiff > 5000 {
		md += fmt.Sprintf("- 💰 Blue team had a massive %s gold lead - significant economic advantage\n", formatNumber(goldDiff))
	} else if goldDiff < -5000 {
		md += fmt.Sprintf("- 💰 Red team had a massive %s gold lead - significant economic advantage\n", formatNumber(-goldDiff))
	}

	killDiff := blueStats.Kills - redStats.Kills
	if killDiff > 10 {
		md += "- ⚔️ Blue team dominated team fights with a significant kill lead\n"
	} else if killDiff < -10 {
		md += "- ⚔️ Red team dominated team fights with a significant kill lead\n"
	}

	visionDiff := blueStats.Vision - redStats.Vision
	if visionDiff > 50 {
		md += "- 👁️ Blue team had superior vision control - major map awareness advantage\n"
	} else if visionDiff < -50 {
		md += "- 👁️ Red team had superior vision control - major map awareness advantage\n"
	}

	// Damage efficiency
	blueDmgPerGold := float64(blueStats.Damage) / float64(blueStats.Gold)
	redDmgPerGold := float64(redStats.Damage) / float64(redStats.Gold)

	if blueDmgPerGold > redDmgPerGold*1.1 {
		md += "- 📈 Blue team was more efficient with their gold (better damage conversion)\n"
	} else if redDmgPerGold > blueDmgPerGold*1.1 {
		md += "- 📈 Red team was more efficient with their gold (better damage conversion)\n"
	}

	md += "\n"

	return md
}

func generateFunFacts(md string, participants []riot.Participant, gameDuration int) string {
	// Most deaths
	maxDeaths := 0
	var mostDeathsPlayer *riot.Participant
	for i := range participants {
		if participants[i].Deaths > maxDeaths {
			maxDeaths = participants[i].Deaths
			mostDeathsPlayer = &participants[i]
		}
	}

	if mostDeathsPlayer != nil && maxDeaths > 7 {
		md += fmt.Sprintf("- Death Magnet: %s died %d times (RIP)\n",
			mostDeathsPlayer.RiotIDGameName, maxDeaths)
	}

	// Most kills
	maxKills := 0
	var mostKillsPlayer *riot.Participant
	for i := range participants {
		if participants[i].Kills > maxKills {
			maxKills = participants[i].Kills
			mostKillsPlayer = &participants[i]
		}
	}

	if mostKillsPlayer != nil && maxKills > 0 {
		md += fmt.Sprintf("- Kill Leader: %s with %d kills\n",
			mostKillsPlayer.RiotIDGameName, maxKills)
	}

	// Richest player
	maxGold := 0
	var richestPlayer *riot.Participant
	for i := range participants {
		if participants[i].GoldEarned > maxGold {
			maxGold = participants[i].GoldEarned
			richestPlayer = &participants[i]
		}
	}

	if richestPlayer != nil {
		md += fmt.Sprintf("- Richest Player: %s earned %s gold\n",
			richestPlayer.RiotIDGameName, formatNumber(maxGold))
	}

	// Best KDA
	bestKDA := 0.0
	var bestKDAPlayer *riot.Participant
	for i := range participants {
		kda := 0.0
		if participants[i].Deaths > 0 {
			kda = float64(participants[i].Kills+participants[i].Assists) / float64(participants[i].Deaths)
		} else if participants[i].Kills+participants[i].Assists > 0 {
			kda = float64(participants[i].Kills + participants[i].Assists)
		}

		if kda > bestKDA {
			bestKDA = kda
			bestKDAPlayer = &participants[i]
		}
	}

	if bestKDAPlayer != nil && bestKDA > 3 {
		kdaStr := "Perfect"
		if bestKDAPlayer.Deaths > 0 {
			kdaStr = fmt.Sprintf("%.2f", bestKDA)
		}
		md += fmt.Sprintf("- Best KDA: %s (%s)\n",
			bestKDAPlayer.RiotIDGameName, kdaStr)
	}

	// Vision king
	maxVision := 0
	var visionKing *riot.Participant
	for i := range participants {
		if participants[i].VisionScore > maxVision {
			maxVision = participants[i].VisionScore
			visionKing = &participants[i]
		}
	}

	if visionKing != nil && maxVision > 0 {
		md += fmt.Sprintf("- Vision King: %s with %d vision score\n",
			visionKing.RiotIDGameName, maxVision)
	}

	// Tank award (most damage taken)
	maxDamageTaken := 0
	var tankPlayer *riot.Participant
	for i := range participants {
		if participants[i].TotalDamageTaken > maxDamageTaken {
			maxDamageTaken = participants[i].TotalDamageTaken
			tankPlayer = &participants[i]
		}
	}

	if tankPlayer != nil {
		md += fmt.Sprintf("- Human Shield: %s absorbed %s damage\n",
			tankPlayer.RiotIDGameName, formatNumber(maxDamageTaken))
	}

	// CS leader
	maxCS := 0
	var csLeader *riot.Participant
	for i := range participants {
		totalCS := participants[i].TotalMinionsKilled + participants[i].NeutralMinionsKilled
		if totalCS > maxCS {
			maxCS = totalCS
			csLeader = &participants[i]
		}
	}

	if csLeader != nil {
		md += fmt.Sprintf("- Farming Simulator: %s with %d CS (%.1f/min)\n",
			csLeader.RiotIDGameName, maxCS, float64(maxCS)/float64(gameDuration)*60)
	}

	// Longest life
	longestLife := 0
	var survivorPlayer *riot.Participant
	for i := range participants {
		if participants[i].LongestTimeSpentLiving > longestLife {
			longestLife = participants[i].LongestTimeSpentLiving
			survivorPlayer = &participants[i]
		}
	}

	if survivorPlayer != nil && longestLife > 600 {
		md += fmt.Sprintf("- Survivor: %s stayed alive for %s straight\n",
			survivorPlayer.RiotIDGameName, formatDuration(longestLife))
	}

	// Most time dead
	maxTimeDead := 0
	var ghostPlayer *riot.Participant
	for i := range participants {
		if participants[i].TotalTimeSpentDead > maxTimeDead {
			maxTimeDead = participants[i].TotalTimeSpentDead
			ghostPlayer = &participants[i]
		}
	}

	if ghostPlayer != nil && maxTimeDead > 300 {
		deadPercent := float64(maxTimeDead) / float64(gameDuration) * 100
		md += fmt.Sprintf("- Gray Screen Simulator: %s spent %s dead (%.1f%% of game)\n",
			ghostPlayer.RiotIDGameName, formatDuration(maxTimeDead), deadPercent)
	}

	md += "\n"

	return md
}

// Helper function to group matches by month
func groupMatchesByMonth(matches []riot.GetMatchResponse) map[string][]riot.GetMatchResponse {
	grouped := make(map[string][]riot.GetMatchResponse)
	for _, match := range matches {
		// Extract the month and year from the match creation timestamp
		timestamp := match.Info.GameCreation
		t := time.Unix(timestamp/1000, 0)
		monthYear := t.Format("2006-01") // Format as YYYY-MM (e.g., "2023-11")

		grouped[monthYear] = append(grouped[monthYear], match)
	}
	return grouped
}

// Function to generate markdown for the player's matches
func GenerateMarkdown(account *riot.GetByRiotIdResponse, matches []riot.GetMatchResponse) error {
	dirPath := strings.ToLower(account.GameName+"_"+account.TagLine) + "_markdown"

	// Create directory for the player's matches
	if err := os.MkdirAll(dirPath, 0755); err != nil {
		logger.Error("Failed to create directory: %v", err)
		return err
	}

	// Group matches by month
	groupedMatches := groupMatchesByMonth(matches)

	// Iterate over the grouped matches for each month
	for _, monthMatches := range groupedMatches {
		// Ensure at least 1 win and 1 loss for the month
		wins := []riot.GetMatchResponse{}
		losses := []riot.GetMatchResponse{}
		for _, match := range monthMatches {
			// Iterate over the participants to check if the player won or lost
			for _, p := range match.Info.Participants {
				// && p.RiotIDTagline == account.TagLine
				if p.RiotIDGameName == account.GameName {
					if p.Win {
						wins = append(wins, match)
					} else {
						losses = append(losses, match)
					}
					break
				}
			}
		}

		// Ensure we have at least 1 win and 1 loss, and then pick up to 3 matches for the month
		if len(wins) > 0 && len(losses) > 0 {
			selectedMatches := []riot.GetMatchResponse{}

			// Ensure we have at least 1 win and 1 loss in the selected matches
			selectedMatches = append(selectedMatches, wins[0], losses[0])

			// Add more matches if we have fewer than 3
			remainingMatches := append(wins[1:], losses[1:]...) // Combine remaining wins and losses
			for i := 0; i < 3-len(selectedMatches) && i < len(remainingMatches); i++ {
				selectedMatches = append(selectedMatches, remainingMatches[i])
			}

			// Generate markdown and metadata for the selected matches
			for _, match := range selectedMatches {
				for _, p := range match.Info.Participants {
					if p.RiotIDGameName == account.GameName && p.RiotIDTagline == account.TagLine {
						markdown := generateMarkdown(p, match)

						fname := fmt.Sprintf("%s/%s_%s_%s_match_report.md",
							dirPath, match.Metadata.MatchID, p.RiotIDGameName, p.RiotIDTagline)

						// Write the markdown file
						if err := os.WriteFile(fname, []byte(markdown), 0644); err != nil {
							fmt.Fprintf(os.Stderr, "Error writing file: %v\n", err)
							os.Exit(1)
						}

						// Prepare metadata JSON
						playerMatchResult := "Won"
						if !p.Win {
							playerMatchResult = "Lost"
						}

						mdJson := MetadataJSON{
							MetadataAttributes: struct {
								MatchID        string "json:\"match_id\""
								MatchMonth     string "json:\"match_month\""
								MatchResult    string "json:\"match_result\""
								RiotIdGameName string "json:\"riotidgamename\""
								RiotIdTagLine  string "json:\"riotidtagline\""
							}{
								MatchID:        match.Metadata.MatchID,
								MatchMonth:     formatTimestampMonth(match.Info.GameCreation),
								RiotIdGameName: p.RiotIDGameName,
								RiotIdTagLine:  p.RiotIDTagline,
								MatchResult:    playerMatchResult,
							},
						}

						// Write metadata JSON file
						file, err := os.Create(fname + ".metadata.json")
						if err != nil {
							logger.Error("Failed to create file: %v", err)
							return err
						}
						encoder := json.NewEncoder(file)
						encoder.SetIndent("", "  ")
						if err := encoder.Encode(mdJson); err != nil {
							logger.Error("Failed to write JSON to file: %v", err)
							return err
						}
						file.Close()
					}
				}
			}
		}
	}

	return nil
}
