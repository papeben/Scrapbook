package main

import (
	"database/sql"
	"fmt"
	"net/http"
)

type scrapbookPage struct {
	Title string
	URI   string
}

func httpHandler(response http.ResponseWriter, request *http.Request) {
	var (
		Title string
		page  scrapbookPage
	)

	// Get page info from database
	err := db.QueryRow("SELECT page_title FROM scrapbook_data.pages WHERE page_uri = $1", request.RequestURI).Scan(&Title)
	if err == sql.ErrNoRows {
		page = scrapbookPage{
			"Page Not Found",
			request.RequestURI,
		}
		response.WriteHeader(404)
	} else if err != sql.ErrNoRows && err != nil {
		logMessage(2, err.Error())
	} else {
		page = scrapbookPage{
			Title,
			request.RequestURI,
		}
	}

	err = formTemplate.Execute(response, page)
	if err != nil {
		fmt.Fprintf(response, "Error.")
	}
	logMessage(4, fmt.Sprintf("%s: %s", request.RemoteAddr, request.RequestURI))
}
