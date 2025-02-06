package main

import (
	"crypto/rand"
	"database/sql"
	"fmt"
	"math/big"
	"os"
	"strconv"
	"time"

	_ "github.com/lib/pq"
)

var (
	logVerbosity  = envWithDefaultInt("LOG_LEVEL", 5)
	POSTGRES_HOST = envWithDefault("POSTGRES_HOST", "127.0.0.1")
	POSTGRES_USER = envWithDefault("POSTGRES_USER", "postgres")
	POSTGRES_PASS = envWithDefault("POSTGRES_PASS", "postgres")
	POSTGRES_DB   = envWithDefault("POSTGRES_DB", "scrapbook")
	POSTGRES_SSL  = envWithDefault("POSTGRES_SSL", "disable")
	EDIT_PASSWORD = envWithDefault("EDIT_PASSWORD", "changeme")
	sevMap        = [6]string{"FATAL", "CRITICAL", "ERROR", "WARNING", "INFO", "DEBUG"}
	db            *sql.DB
)

func main() {
	err := dbConnect(5)
	if err != nil {
		logMessage(0, fmt.Sprintf("Failed to connect to create database connection: %s", err.Error()))
		os.Exit(1)
	}
}

func envWithDefault(variableName string, defaultString string) string {
	val := os.Getenv(variableName)
	if len(val) == 0 {
		return defaultString
	} else {
		logMessage(5, fmt.Sprintf("Loaded %s value '%s'", variableName, val))
		return val
	}
}

func envWithDefaultInt(variableName string, defaultInt int) int {
	val := os.Getenv(variableName)
	if len(val) == 0 {
		return defaultInt
	} else {
		i, err := strconv.Atoi(val)
		if err != nil {
			fmt.Printf("[CRITICAL] Integer parameter %s is not valid\n", val)
			os.Exit(1)
		}
		return i
	}
}

func logMessage(severity int, message string) {
	var moment = time.Now()

	if severity <= logVerbosity {
		fmt.Printf("[%s] %02d:%02d:%02d %04d-%02d-%02d %s\n", sevMap[severity], moment.Hour(), moment.Minute(), moment.Second(), moment.Year(), moment.Month(), moment.Day(), message)
	}
}

func GenerateRandomString(length int) string {
	charset := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	charsetLength := big.NewInt(int64(len(charset)))
	bytes := make([]byte, length)

	for i := 0; i < length; i++ {
		num, err := rand.Int(rand.Reader, charsetLength)
		if err != nil {
			logMessage(0, fmt.Sprintf("Failed to generate random number: %s", err.Error()))
			os.Exit(1)
		}

		bytes[i] = charset[num.Int64()]
	}

	return string(bytes)
}

func dbConnect(n int) error {
	var err error
	db, err = sql.Open("postgres", fmt.Sprintf("postgres://%s:%s@%s/%s?sslmode=%s", POSTGRES_USER, POSTGRES_PASS, POSTGRES_HOST, POSTGRES_DB, POSTGRES_SSL))
	if err != nil {
		return err
	}

	_, err = db.Exec("CREATE SCHEMA IF NOT EXISTS scrapbook_internal")
	if err != nil && n > 1 {
		logMessage(2, fmt.Sprintf("Failed to create database connection: %s", err.Error()))
		logMessage(2, "Retrying connection in 5 seconds...")
		time.Sleep(5 * time.Second)
		err = dbConnect(n - 1)
		if err != nil {
			return err
		}
	} else if err != nil && n <= 1 {
		return err
	} else {
		logMessage(5, "Schema created")
	}

	// Test DB
	err = db.QueryRow("SELECT version FROM configuration").Scan()
	if err == nil {
		return err
	}

	_, err = db.Exec("CREATE TABLE IF NOT EXISTS scrapbook_internal.configuration (db_version VARCHAR(16) NOT NULL)")
	if err != nil {
		return err
	}

	return nil
}
