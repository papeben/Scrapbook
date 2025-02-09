package main

import (
	"database/sql"
	"fmt"
	"net/http"
)

func isSessionAuthenticated(response http.ResponseWriter, request *http.Request) bool {
	sessionCookie, err := request.Cookie(EDIT_COOKIE)
	if err != nil {
		response.WriteHeader(401)
		fmt.Fprintf(response, "Error.")
		return false
	}

	var token string
	err = db.QueryRow("SELECT session_id FROM scrapbook_data.editors WHERE session_id = $1 AND timestamp > now() - INTERVAL '1 DAY'", sessionCookie.Value).Scan(&token)
	if err == sql.ErrNoRows {
		response.WriteHeader(401)
		fmt.Fprintf(response, "Error.")
		return false
	} else if err != nil {
		logMessage(2, err.Error())
		response.WriteHeader(500)
		fmt.Fprintf(response, "Error.")
		return false
	}
	return true
}
