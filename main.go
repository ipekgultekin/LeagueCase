package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"sort"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

type Team struct {
	Name     string
	Strength int
}

type Match struct {
	HomeTeam  *Team
	AwayTeam  *Team
	HomeGoals int
	AwayGoals int
	Played    bool
}

type Standing struct {
	TeamName       string
	Played         int
	Wins           int
	Draws          int
	Losses         int
	GoalsFor       int
	GoalsAgainst   int
	GoalDifference int
	Points         int
}

type DBMatch struct {
	ID        int    `json:"id"`
	HomeTeam  string `json:"home_team"`
	AwayTeam  string `json:"away_team"`
	HomeGoals int    `json:"home_goals"`
	AwayGoals int    `json:"away_goals"`
	Played    bool   `json:"played"`
}


var db *sql.DB
var teams = []*Team{
	{"Alpha FC", 85},
	{"Bravo United", 70},
	{"Charlie Town", 60},
	{"Delta SC", 50},
}
var fixture = generateFixture(teams)

func initDatabase() {
	var err error
	db, err = sql.Open("sqlite3", "./league.db")
	if err != nil {
		panic(err)
	}

	createTeams := `
	CREATE TABLE IF NOT EXISTS teams (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT,
		strength INTEGER
	);`

	createMatches := `
	CREATE TABLE IF NOT EXISTS matches (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		home_team TEXT,
		away_team TEXT,
		home_goals INTEGER,
		away_goals INTEGER,
		played BOOLEAN
	);`

	if _, err = db.Exec(createTeams); err != nil {
		panic(err)
	}
	if _, err = db.Exec(createMatches); err != nil {
		panic(err)
	}
}

func generateFixture(teams []*Team) []Match {
	var matches []Match
	for i := 0; i < len(teams); i++ {
		for j := 0; j < len(teams); j++ {
			if i != j {
				match := Match{
					HomeTeam: teams[i],
					AwayTeam: teams[j],
					Played:   false,
				}
				matches = append(matches, match)
			}
		}
	}
	return matches
}

func simulateMatch(match *Match) {
	rand.Seed(time.Now().UnixNano())

	homeAdvantage := 10
	homeStrength := match.HomeTeam.Strength + homeAdvantage
	awayStrength := match.AwayTeam.Strength

	match.HomeGoals = rand.Intn((homeStrength / 20) + 1)
	match.AwayGoals = rand.Intn((awayStrength / 20) + 1)
	match.Played = true
}

func calculateStandings(teams []*Team, matches []Match) []Standing {
	standingsMap := make(map[string]*Standing)
	for _, team := range teams {
		standingsMap[team.Name] = &Standing{TeamName: team.Name}
	}
	for _, match := range matches {
		if !match.Played {
			continue
		}
		home := standingsMap[match.HomeTeam.Name]
		away := standingsMap[match.AwayTeam.Name]

		home.Played++
		away.Played++

		home.GoalsFor += match.HomeGoals
		home.GoalsAgainst += match.AwayGoals

		away.GoalsFor += match.AwayGoals
		away.GoalsAgainst += match.HomeGoals

		if match.HomeGoals > match.AwayGoals {
			home.Wins++
			home.Points += 3
			away.Losses++
		} else if match.HomeGoals < match.AwayGoals {
			away.Wins++
			away.Points += 3
			home.Losses++
		} else {
			home.Draws++
			away.Draws++
			home.Points++
			away.Points++
		}
	}
	var standings []Standing
	for _, s := range standingsMap {
		s.GoalDifference = s.GoalsFor - s.GoalsAgainst
		standings = append(standings, *s)
	}
	sort.SliceStable(standings, func(i, j int) bool {
		if standings[i].Points == standings[j].Points {
			return standings[i].GoalDifference > standings[j].GoalDifference
		}
		return standings[i].Points > standings[j].Points
	})
	return standings
}

func teamsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(teams)
}

func fixtureHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(fixture)
}

func simulateHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Only POST method is allowed", http.StatusMethodNotAllowed)
		return
	}

	for i := range fixture {
		if !fixture[i].Played {
			simulateMatch(&fixture[i])

			// Insert into database (optional for now)
			_, err := db.Exec(
				`INSERT INTO matches (home_team, away_team, home_goals, away_goals, played)
				 VALUES (?, ?, ?, ?, ?)`,
				fixture[i].HomeTeam.Name,
				fixture[i].AwayTeam.Name,
				fixture[i].HomeGoals,
				fixture[i].AwayGoals,
				fixture[i].Played,
			)
			if err != nil {
				fmt.Println("DB Insert error:", err)
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": "All matches have been simulated!"})
}

func standingsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	standings := calculateStandings(teams, fixture)
	json.NewEncoder(w).Encode(standings)
}

func matchesHandler(w http.ResponseWriter, r *http.Request) {
	rows, err := db.Query("SELECT id, home_team, away_team, home_goals, away_goals, played FROM matches")
	if err != nil {
		http.Error(w, "Failed to fetch matches from database", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var matches []DBMatch
	for rows.Next() {
		var m DBMatch
		err := rows.Scan(&m.ID, &m.HomeTeam, &m.AwayTeam, &m.HomeGoals, &m.AwayGoals, &m.Played)
		if err != nil {
			http.Error(w, "Error while scanning match row", http.StatusInternalServerError)
			return
		}
		matches = append(matches, m)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(matches)
}


func main() {
	initDatabase()

	http.HandleFunc("/teams", teamsHandler)
	http.HandleFunc("/fixture", fixtureHandler)
	http.HandleFunc("/simulate", simulateHandler)
	http.HandleFunc("/standings", standingsHandler)
	http.HandleFunc("/matches", matchesHandler)

	fmt.Println("Server is running on http://localhost:8080")
	http.ListenAndServe(":8080", nil)
}
