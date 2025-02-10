package main

import (
	"crypto/rand"
	"database/sql"
	"fmt"
	"html/template"
	"math/big"
	"net/http"
	"os"
	"strconv"
	"time"

	_ "github.com/lib/pq"
)

var (
	logVerbosity          = envWithDefaultInt("LOG_LEVEL", 5)
	POSTGRES_HOST         = envWithDefault("POSTGRES_HOST", "127.0.0.1")
	POSTGRES_USER         = envWithDefault("POSTGRES_USER", "postgres")
	POSTGRES_PASS         = envWithDefault("POSTGRES_PASS", "postgres")
	POSTGRES_DB           = envWithDefault("POSTGRES_DB", "scrapbook")
	POSTGRES_SSL          = envWithDefault("POSTGRES_SSL", "disable")
	EDIT_PASSWORD         = envWithDefault("EDIT_PASSWORD", "changeme")
	EDIT_COOKIE           = envWithDefault("EDIT_COOKIE", "scrapbook-edit")
	HTTP_PORT             = envWithDefault("HTTP_PORT", "8080")
	MEDIA_DIRECTORY       = envWithDefault("MEDIA_WEB_DIR", "/media")
	TEMP_DIRECTORY        = envWithDefault("TEMP_DIR", "/tmp")
	sevMap                = [6]string{"FATAL", "CRITICAL", "ERROR", "WARNING", "INFO", "DEBUG"}
	imageResolutionSteps  = [10]int{144, 240, 360, 480, 576, 720, 960, 1080, 1440, 2160}
	videoBitrateSteps     = [10]float32{0.2, 0.4, 0.6, 0.8, 1.2, 2, 3, 4, 8, 14}
	videoFFMPEGPreset     = "slow"
	videoFFMPEGCodec      = "libvpx"
	videoFFMPEGAudioCodec = "libvorbis"
	videoFFMPEGContainer  = "webm"
	db                    *sql.DB
	formTemplate          *template.Template
)

func main() {
	loadTemplates()
	err := dbConnect(5)
	if err != nil {
		logMessage(0, fmt.Sprintf("Failed to connect to create database connection: %s", err.Error()))
		os.Exit(1)
	}

	http.HandleFunc("/", httpHandler)
	server := &http.Server{
		Addr:              ":" + HTTP_PORT,
		ReadHeaderTimeout: 10 * time.Second,
	}
	logMessage(4, fmt.Sprintf("Listening for incoming requests on %s", server.Addr))
	err = server.ListenAndServe()
	if err != nil {
		logMessage(0, fmt.Sprintf("Server error: %s", err.Error()))
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

func generateRandomString(length int) string {
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

func loadTemplates() {
	var err error
	formTemplate, err = template.ParseFiles("scrapbook.html") // Preload form page template into memory
	if err != nil {
		logMessage(0, fmt.Sprintf("Unable to load HTML template from scrapbook.html: %s", err.Error()))
		os.Exit(1)
	}
}

func errWithWeb(err error, response http.ResponseWriter, statusMessage string) {
	logMessage(1, err.Error())
	logMessage(5, statusMessage)
	response.WriteHeader(500)
	fmt.Fprintf(response, "An internal error occurred.")
}
