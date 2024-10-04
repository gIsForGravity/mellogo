package db

import (
	"database/sql"
	"fmt"
	"time"
)

type Database struct {
	driver string
	url    string
}

func (db Database) Open() (*Connection, error) {
	driver, err := sql.Open(db.driver, db.url)
	if err != nil {
		return nil, err
	}
	return &Connection{db, driver}, nil
}

func (db Database) Initialize() error {
	// open connection
	conn, err := db.Open()
	if err != nil {
		return err
	}
	defer conn.Close()

	// scores table | minutes int | seconds int | year int | month int | day int | userid string |
	query := `
	CREATE TABLE IF NOT EXISTS scores
	(minutes INT, seconds INT, year INT, month INT, day INT, userid TEXT);`
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
	db     Database
	driver *sql.DB
}

func (conn Connection) Close() error {
	return conn.driver.Close()
}

func (conn Connection) SubmitScore(userid string, minutes int, seconds int, date time.Time) error {
	// start transaction
	tx, err := conn.driver.Begin()
	if err != nil {
		return err
	}

	// insert
	insertion := fmt.Sprintf(`
	INSERT INTO scores (minutes, seconds, year, month, day, userid)
	VALUES (%d, %d, %d, %d, %d, \"%s\")`, minutes, seconds, date.Year(), date.Month(), date.Day(), userid)
	_, err = tx.Exec(insertion)
	if err != nil {
		return err
	}

	// end transaction
	return tx.Commit()
}
