package db

import (
	"database/sql"
	"fmt"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

type Database struct {
	driver string
	url    string
}

func CreateSqlite(path string) Database {
	return Database{
		driver: "sqlite3",
		url:    path,
	}
}

func (db Database) Open() (*Connection, error) {
	driver, err := sql.Open(db.driver, db.url)
	if err != nil {
		return nil, err
	}
	return &Connection{driver}, nil
}

func (db Database) Initialize() error {
	// open connection
	conn, err := db.Open()
	if err != nil {
		return err
	}
	defer conn.Close()

	// scores table | seconds int | year int | month int | day int | userid string |
	query := `
	CREATE TABLE IF NOT EXISTS scores
	(minutes INT, seconds INT, year INT, month INT, day INT, userid TEXT)`
	_, err = conn.driver.Exec(query)
	if err != nil {
		return err
	}

	// users table | userid string primary | nickname string |
	query = `
	CREATE TABLE IF NOT EXISTS users
	(userid TEXT PRIMARY KEY, nickname TEXT)`
	_, err = conn.driver.Exec(query)
	if err != nil {
		return err
	}

	return nil
}

type Connection struct {
	// db     Database // this doesn't seem to be needed
	driver *sql.DB
}

func (conn Connection) Close() error {
	return conn.driver.Close()
}

func (conn Connection) SubmitScore(userid string, defaultUsername string, minutes int, seconds int, date time.Time) error {
	// start transaction
	tx, err := conn.driver.Begin()
	if err != nil {
		return err
	}

	// insert entry
	insertion := fmt.Sprintf(`
	INSERT INTO scores (minutes, seconds, year, month, day, userid)
	VALUES (%d, %d, %d, %d, %d, "%s")`, minutes, seconds, date.Year(), date.Month(), date.Day(), userid)
	_, err = tx.Exec(insertion)
	if err != nil {
		tx.Rollback()
		return err
	}
	// insert default username
	_, err = tx.Exec(`
	INSERT INTO users 
		(userid, nickname)
	VALUES
		(?, ?)
	ON CONFLICT
		(userid)
	DO NOTHING`, userid, defaultUsername)
	if err != nil {
		tx.Rollback()
		return err
	}

	// end transaction
	return tx.Commit()
}

func (conn Connection) SetNickname(userid string, nickname string) error {
	_, err := conn.driver.Exec(`
	INSERT INTO users
		(userid, nickname)
	VALUES
		(?, ?)
	ON CONFLICT
		(userid)
	DO UPDATE SET
		nickname = excluded.nickname`)

	return err
}

type ScoreResult struct {
	Minutes int
	Seconds int
	Date    time.Time
	User    string
}

func (conn Connection) QueryTopScores(count int) ([]ScoreResult, error) {
	query := fmt.Sprintf(`
	SELECT 
		scores.minutes, scores.seconds, scores.year, scores.month, scores.day, users.nickname
	FROM scores
	INNER JOIN users
	ON scores.userid = users.userid
	ORDER BY scores.minutes, scores.seconds
	LIMIT %d`,
		count)

	// execute query
	result, err := conn.driver.Query(query)
	if err != nil {
		return nil, err
	}
	defer result.Close()

	// fmt.Printf("QueryTopScores: result: %+v\n", result)

	scores := make([]ScoreResult, 0, count)
	for result.Next() {
		var r ScoreResult
		var year int
		var month int
		var day int
		err := result.Scan(&r.Minutes, &r.Seconds, &year, &month, &day, &r.User)
		if err != nil {
			return nil, err
		}
		// collapse year month day into date
		r.Date = time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.Local)
		// add result to list
		scores = append(scores, r)
		fmt.Printf("QueryTopScores: last scanned score: %+v", r)
	}

	return scores, nil
}
