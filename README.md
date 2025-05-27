# League Case ⚽
Hi! This repo includes my backend solution for the Insider Development Case.

## 📌 About the Project
This project simulates a mini football league with 4 teams.  
All matches are simulated, and the league table updates accordingly. The whole process is accessible via REST API.

### 🧠 League Rules
- 4 teams play each other twice (home & away) → 12 matches total  
- Win = 3 pts, Draw = 1 pt, Loss = 0 pts  
- Tiebreaker is goal difference

---

## 🛠️ Technologies
- **Language:** Go (Golang)
- **Database:** SQLite  
- **HTTP Server:** Go's built-in `net/http`  
- **DB Driver:** `github.com/mattn/go-sqlite3`

---

## 🌐 API Endpoints

| Method | Endpoint               | Description                             |
|--------|------------------------|-----------------------------------------|
| GET    | `/teams`              | List of all teams                       |
| GET    | `/matches`            | List of all matches                     |
| GET    | `/matches?week=n`     | Matches of specific week                |
| POST   | `/simulate/week/{n}`  | Simulates matches of week n             |
| POST   | `/simulate/all`       | Simulates all remaining matches         |
| GET    | `/standings`          | Returns current league standings        |
| GET    | `/predict`            | Predicts final league standings         |
| POST   | `/match/update`       | Manually update a match result          |

---

## 🧪 How to Run
1. Make sure you have Go installed  
2. Install the SQLite driver:
    ```bash
    go get github.com/mattn/go-sqlite3
    ```
3. Run the project:
    ```bash
    go run main.go
    ```
4. Test endpoints via browser or Postman:
    - `http://localhost:8080/teams`
    - `http://localhost:8080/matches`
    - `http://localhost:8080/simulate/week/1`
    - `http://localhost:8080/standings`
    - etc.

---

## 💾 Database
- A file called `league.db` is created automatically  
- Tables used: `teams` and `matches`  
- You can check the structure in `schema.sql`

---

## 🙋‍♀️ Author
- İpek Gültekin  
