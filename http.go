package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

type scrapbookSitemap struct {
	Pages []scrapbookPage
}

type scrapbookPage struct {
	Header   scrapbookPageHeader
	Elements []scrapbookElement
}

type scrapbookElement struct {
	ID        string
	Name      string
	StyleID   string
	PosAnchor string
	PosX      float32
	PosY      float32
	PosZ      int
	Width     float32
	Height    float32
	IsLink    bool
	LinkURL   string
	Content   string
	Children  []scrapbookElement
}

type scrapbookStyle struct {
	ID                 string
	Name               string
	BackgroundType     string
	BackgroundData     string
	BackgroundPosition string
	BackgroundSize     string
	FontFamily         string
	FontSize           float32
	FontWeight         string
	FontColor          string
	Margin             float32
	Padding            float32
	TextAlign          string
	BorderWidth        int
	BorderStyle        string
	BorderColor        string
	CustomCSS          string
}

type scrapbookPageHeader struct {
	Title string
	URI   string
}

func httpHandler(response http.ResponseWriter, request *http.Request) {
	var (
		title string
		page  scrapbookPageHeader
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

	// Serve sitemap
	if request.URL.Path == "/sitemap.json" {
		var (
			pages []scrapbookPage
		)

		pageRows, err := db.Query("SELECT page_title, page_uri FROM scrapbook_data.pages")
		if err != nil {
			logMessage(2, err.Error())
			return
		}

		for pageRows.Next() {
			var title, uri string
			pageRows.Scan(&title, &uri)

			pages = append(pages, scrapbookPage{
				scrapbookPageHeader{
					title,
					uri,
				},
				getNestedElements("page", uri),
			})

		}

		jsonBytes, err := json.Marshal(pages)
		if err != nil {
			logMessage(2, err.Error())
			return
		}

		fmt.Fprint(response, string(jsonBytes))

		return
	}

	// Get page info from database
	err := db.QueryRow("SELECT page_title FROM scrapbook_data.pages WHERE page_uri = $1", request.URL.Path).Scan(&title)
	if err == sql.ErrNoRows {
		page = scrapbookPageHeader{
			"Page Not Found",
			request.URL.Path,
		}
		response.WriteHeader(404)
	} else if err != sql.ErrNoRows && err != nil {
		logMessage(2, err.Error())
	} else {
		page = scrapbookPageHeader{
			title,
			request.URL.Path,
		}
	}

	err = formTemplate.Execute(response, page)
	if err != nil {
		logMessage(1, err.Error())
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

func getNestedElements(parentType string, parentId string) []scrapbookElement {
	elementRows, err := db.Query("SELECT element_id, element_name, style_id, pos_anchor, pos_x, pos_y, pos_z, width, height, is_link, link_url, content FROM scrapbook_data.elements WHERE parent_type = $1 AND parent_id = $2 ORDER BY sequence_number ASC", parentType, parentId)
	if err != nil {
		logMessage(2, err.Error())
	}

	var (
		elements     []scrapbookElement = []scrapbookElement{}
		element_id   string
		element_name string
		style_id     string
		pos_anchor   string
		pos_x        float32
		pos_y        float32
		pos_z        int
		width        float32
		height       float32
		is_link      bool
		link_url     string
		content      string
	)

	for elementRows.Next() {
		elementRows.Scan(&element_id, &element_name, &style_id, &pos_anchor, &pos_x, &pos_y, &pos_z, &width, &height, &is_link, &link_url, &content)
		elements = append(elements, scrapbookElement{
			element_id,
			element_name,
			style_id,
			pos_anchor,
			pos_x,
			pos_y,
			pos_z,
			width,
			height,
			is_link,
			link_url,
			content,
			getNestedElements("element", element_id),
		})
	}
	return elements

}
