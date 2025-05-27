package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"sort"
	"strconv"

	_ "github.com/mattn/go-sqlite3"
)

// Interfaces
type MatchSimulator interface {
	Simulate() error
}

type StandingsCalculator interface {
	CalculateStandings() ([]Standing, error)
}

// Team struct 
type Team struct {
	Name     string `json:"name"`
	Strength int    `json:"strength"`
}

// Match struct 
type Match struct {
	ID        int    `json:"id"`
	HomeTeam  string `json:"home_team"`
	AwayTeam  string `json:"away_team"`
	HomeGoals int    `json:"home_goals"`
	AwayGoals int    `json:"away_goals"`
	Played    bool   `json:"played"`
	Week      int    `json:"week"`
}

// Standing struct remains the same
type Standing struct {
	TeamName       string `json:"team_name"`
	Played         int    `json:"played"`
	Wins           int    `json:"wins"`
	Draws          int    `json:"draws"`
	Losses         int    `json:"losses"`
	GoalsFor       int    `json:"goals_for"`
	GoalsAgainst   int    `json:"goals_against"`
	GoalDifference int    `json:"goal_difference"`
	Points         int    `json:"points"`
}

type League struct {
	db     *sql.DB
	teams  []Team
	weeks  int
}

func NewLeague(db *sql.DB, teams []Team, totalWeeks int) *League {
	return &League{
		db:     db,
		teams:  teams,
		weeks:  totalWeeks,
	}
}

func (l *League) InitDatabase() error {
	createTeams := `
	CREATE TABLE IF NOT EXISTS teams (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT UNIQUE,
		strength INTEGER
	);`

	createMatches := `
	CREATE TABLE IF NOT EXISTS matches (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		home_team TEXT,
		away_team TEXT,
		home_goals INTEGER DEFAULT 0,
		away_goals INTEGER DEFAULT 0,
		played BOOLEAN DEFAULT FALSE,
		week INTEGER,
		FOREIGN KEY (home_team) REFERENCES teams(name),
		FOREIGN KEY (away_team) REFERENCES teams(name)
	);`

	if _, err := l.db.Exec(createTeams); err != nil {
		return fmt.Errorf("error creating teams table: %v", err)
	}

	if _, err := l.db.Exec(createMatches); err != nil {
		return fmt.Errorf("error creating matches table: %v", err)
	}

	for _, team := range l.teams {
		_, err := l.db.Exec("INSERT OR IGNORE INTO teams (name, strength) VALUES (?, ?)", 
			team.Name, team.Strength)
		if err != nil {
			return fmt.Errorf("error inserting team: %v", err)
		}
	}

	var count int
	err := l.db.QueryRow("SELECT COUNT(*) FROM matches").Scan(&count)
	if err != nil {
		return fmt.Errorf("error checking matches count: %v", err)
	}

	if count == 0 {
		if err := l.GenerateFixture(); err != nil {
			return fmt.Errorf("error generating fixture: %v", err)
		}
	}

	return nil
}

func (l *League) GenerateFixture() error {
	if _, err := l.db.Exec("DELETE FROM matches"); err != nil {
		return err
	}

	var matches []Match
	teamCount := len(l.teams)
	//totalMatches := teamCount * (teamCount - 1)
	//matchesPerWeek := totalMatches / l.weeks

	for i := 0; i < teamCount; i++ {
		for j := 0; j < teamCount; j++ {
			if i != j {
				week := (i + j) % l.weeks
				if week == 0 {
					week = l.weeks
				}
				matches = append(matches, Match{
					HomeTeam: l.teams[i].Name,
					AwayTeam: l.teams[j].Name,
					Week:     week,
				})
			}
		}
	}
	tx, err := l.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for _, match := range matches {
		_, err := tx.Exec(
			`INSERT INTO matches (home_team, away_team, week) VALUES (?, ?, ?)`,
			match.HomeTeam, match.AwayTeam, match.Week,
		)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (l *League) SimulateWeek(week int) error {
	tx, err := l.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	rows, err := tx.Query("SELECT id, home_team, away_team FROM matches WHERE week = ? AND played = FALSE", week)
	if err != nil {
		return err
	}
	defer rows.Close()

	var matches []Match
	for rows.Next() {
		var m Match
		if err := rows.Scan(&m.ID, &m.HomeTeam, &m.AwayTeam); err != nil {
			return err
		}
		matches = append(matches, m)
	}

	for _, match := range matches {
		// team strengths
		var homeStrength, awayStrength int
		err := tx.QueryRow("SELECT strength FROM teams WHERE name = ?", match.HomeTeam).Scan(&homeStrength)
		if err != nil {
			return err
		}
		err = tx.QueryRow("SELECT strength FROM teams WHERE name = ?", match.AwayTeam).Scan(&awayStrength)
		if err != nil {
			return err
		}

		// Simulate match with home advantage (+10)
		homeAdvantage := 10
		match.HomeGoals = rand.Intn((homeStrength+homeAdvantage)/20 + 1)
		match.AwayGoals = rand.Intn(awayStrength/20 + 1)
		match.Played = true

		// Update match in database
		_, err = tx.Exec(
			`UPDATE matches SET home_goals = ?, away_goals = ?, played = TRUE WHERE id = ?`,
			match.HomeGoals, match.AwayGoals, match.ID,
		)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (l *League) CalculateStandings() ([]Standing, error) {
	// all teams
	rows, err := l.db.Query("SELECT name FROM teams")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	standingsMap := make(map[string]*Standing)
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		standingsMap[name] = &Standing{TeamName: name}
	}

	// all played matches
	matchRows, err := l.db.Query("SELECT home_team, away_team, home_goals, away_goals FROM matches WHERE played = TRUE")
	if err != nil {
		return nil, err
	}
	defer matchRows.Close()

	for matchRows.Next() {
		var homeTeam, awayTeam string
		var homeGoals, awayGoals int
		if err := matchRows.Scan(&homeTeam, &awayTeam, &homeGoals, &awayGoals); err != nil {
			return nil, err
		}

		home := standingsMap[homeTeam]
		away := standingsMap[awayTeam]

		home.Played++
		away.Played++

		home.GoalsFor += homeGoals
		home.GoalsAgainst += awayGoals

		away.GoalsFor += awayGoals
		away.GoalsAgainst += homeGoals

		if homeGoals > awayGoals {
			home.Wins++
			home.Points += 3
			away.Losses++
		} else if homeGoals < awayGoals {
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

	return standings, nil
}

func (l *League) PredictStandings() ([]Standing, error) {
	// Get the current standings
	currentStandings, err := l.CalculateStandings()
	if err != nil {
		return nil, err
	}

	// Get the remaining matches
	rows, err := l.db.Query("SELECT home_team, away_team FROM matches WHERE played = FALSE")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// I create a map for easier access
	teamMap := make(map[string]*Standing)
	for i := range currentStandings {
		teamMap[currentStandings[i].TeamName] = &currentStandings[i]
	}

	// Simulate remaining matches
	for rows.Next() {
		var homeTeam, awayTeam string
		if err := rows.Scan(&homeTeam, &awayTeam); err != nil {
			return nil, err
		}

		// Get team powers
		var homeStrength, awayStrength int
		err := l.db.QueryRow("SELECT strength FROM teams WHERE name = ?", homeTeam).Scan(&homeStrength)
		if err != nil {
			return nil, err
		}
		err = l.db.QueryRow("SELECT strength FROM teams WHERE name = ?", awayTeam).Scan(&awayStrength)
		if err != nil {
			return nil, err
		}

		// Simulate match with home advantage (+10)
		homeAdvantage := 10
		homeGoals := rand.Intn((homeStrength+homeAdvantage)/20 + 1)
		awayGoals := rand.Intn(awayStrength/20 + 1)

		// Update predicted standings
		home := teamMap[homeTeam]
		away := teamMap[awayTeam]

		home.Played++
		away.Played++

		home.GoalsFor += homeGoals
		home.GoalsAgainst += awayGoals

		away.GoalsFor += awayGoals
		away.GoalsAgainst += homeGoals

		if homeGoals > awayGoals {
			home.Wins++
			home.Points += 3
			away.Losses++
		} else if homeGoals < awayGoals {
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

	// Calculate goal differences
	for i := range currentStandings {
		currentStandings[i].GoalDifference = currentStandings[i].GoalsFor - currentStandings[i].GoalsAgainst
	}

	// Sorting
	sort.SliceStable(currentStandings, func(i, j int) bool {
		if currentStandings[i].Points == currentStandings[j].Points {
			return currentStandings[i].GoalDifference > currentStandings[j].GoalDifference
		}
		return currentStandings[i].Points > currentStandings[j].Points
	})

	return currentStandings, nil
}

func (l *League) UpdateMatchResult(matchID, homeGoals, awayGoals int) error {
	tx, err := l.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// I get the current result to calculate the difference
	var currentHomeGoals, currentAwayGoals int
	var played bool
	err = tx.QueryRow("SELECT home_goals, away_goals, played FROM matches WHERE id = ?", matchID).
		Scan(&currentHomeGoals, &currentAwayGoals, &played)
	if err != nil {
		return err
	}

	// Update the match
	_, err = tx.Exec(
		`UPDATE matches SET home_goals = ?, away_goals = ?, played = TRUE WHERE id = ?`,
		homeGoals, awayGoals, matchID,
	)
	if err != nil {
		return err
	}

	return tx.Commit()
}

func main() {
	// Initialize teams
	teams := []Team{
		{"Alpha FC", 85},
		{"Bravo United", 70},
		{"Charlie Town", 60},
		{"Delta SC", 50},
	}

	// Open database
	db, err := sql.Open("sqlite3", "./league.db")
	if err != nil {
		panic(fmt.Errorf("failed to open database: %v", err))
	}
	defer db.Close()

	// Assume that league with 6 weeks
	league := NewLeague(db, teams, 6)
	if err := league.InitDatabase(); err != nil {
		panic(fmt.Errorf("failed to initialize database: %v", err))
	}

	// HTTP Handlers
	http.HandleFunc("/teams", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(teams)
	})

	http.HandleFunc("/matches", func(w http.ResponseWriter, r *http.Request) {
		weekStr := r.URL.Query().Get("week")
		var rows *sql.Rows
		var err error

		if weekStr != "" {
			week, err := strconv.Atoi(weekStr)
			if err != nil {
				http.Error(w, "Invalid week parameter", http.StatusBadRequest)
				return
			}
			rows, err = db.Query("SELECT id, home_team, away_team, home_goals, away_goals, played, week FROM matches WHERE week = ?", week)
		} else {
			rows, err = db.Query("SELECT id, home_team, away_team, home_goals, away_goals, played, week FROM matches")
		}

		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		var matches []Match
		for rows.Next() {
			var m Match
			if err := rows.Scan(&m.ID, &m.HomeTeam, &m.AwayTeam, &m.HomeGoals, &m.AwayGoals, &m.Played, &m.Week); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			matches = append(matches, m)
		}

		json.NewEncoder(w).Encode(matches)
	})

	http.HandleFunc("/simulate/week/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		weekStr := r.URL.Path[len("/simulate/week/"):]
		week, err := strconv.Atoi(weekStr)
		if err != nil {
			http.Error(w, "Invalid week", http.StatusBadRequest)
			return
		}

		if err := league.SimulateWeek(week); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		json.NewEncoder(w).Encode(map[string]string{"message": fmt.Sprintf("Week %d simulated successfully", week)})
	})

	http.HandleFunc("/simulate/all", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		for week := 1; week <= league.weeks; week++ {
			if err := league.SimulateWeek(week); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		}

		json.NewEncoder(w).Encode(map[string]string{"message": "All weeks simulated successfully"})
	})

	http.HandleFunc("/standings", func(w http.ResponseWriter, r *http.Request) {
		standings, err := league.CalculateStandings()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		json.NewEncoder(w).Encode(standings)
	})

	http.HandleFunc("/predict", func(w http.ResponseWriter, r *http.Request) {
		standings, err := league.PredictStandings()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		json.NewEncoder(w).Encode(standings)
	})

	http.HandleFunc("/match/update", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var match struct {
			ID        int `json:"id"`
			HomeGoals int `json:"home_goals"`
			AwayGoals int `json:"away_goals"`
		}

		if err := json.NewDecoder(r.Body).Decode(&match); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		if err := league.UpdateMatchResult(match.ID, match.HomeGoals, match.AwayGoals); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		json.NewEncoder(w).Encode(map[string]string{"message": "Match updated successfully"})
	})

	fmt.Println("Server running on http://localhost:8080")
	http.ListenAndServe(":8080", nil)
}