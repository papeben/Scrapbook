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
