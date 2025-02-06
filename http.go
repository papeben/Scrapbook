package main

import (
	"database/sql"
	"fmt"
	"net/http"
	"net/url"
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

	// Edit API
	if request.URL.Path == "/editapi/requestedit" {
		query, err := url.ParseQuery(request.URL.RawQuery)
		if err != nil {
			response.WriteHeader(500)
			fmt.Fprint(response, "Error.")
			return
		}

		if query.Get("p") != EDIT_PASSWORD {
			response.WriteHeader(401)
			fmt.Fprint(response, "Invalid.")
			return
		}

		logMessage(4, fmt.Sprintf("Editor %s authenticated", request.RemoteAddr))
		token, err := createEditorSession()
		if err != nil {
			response.WriteHeader(500)
			fmt.Fprint(response, "Error.")
			return
		}

		cookie := http.Cookie{Name: EDIT_COOKIE, Value: token, SameSite: http.SameSiteStrictMode, Secure: false, Path: "/"}
		http.SetCookie(response, &cookie)
		response.WriteHeader(200)
		fmt.Fprint(response, "Ok.")
		return

	}

	// Get page info from database
	err := db.QueryRow("SELECT page_title FROM scrapbook_data.pages WHERE page_uri = $1", request.URL.Path).Scan(&Title)
	if err == sql.ErrNoRows {
		page = scrapbookPage{
			"Page Not Found",
			request.URL.Path,
		}
		response.WriteHeader(404)
	} else if err != sql.ErrNoRows && err != nil {
		logMessage(2, err.Error())
	} else {
		page = scrapbookPage{
			Title,
			request.URL.Path,
		}
	}

	err = formTemplate.Execute(response, page)
	if err != nil {
		fmt.Fprintf(response, "Error.")
	}
	logMessage(4, fmt.Sprintf("%s: %s", request.RemoteAddr, request.RequestURI))
}

func createEditorSession() (string, error) {
	newToken := generateRandomString(256)
	err := db.QueryRow("SELECT session_id FROM scrapbook_data.editors WHERE session_id = $1", newToken).Scan()
	if err == nil {
		return createEditorSession()
	} else if err == sql.ErrNoRows {
		return newToken, nil
	}
	return "", err
}
