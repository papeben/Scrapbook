package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"html/template"
	"image"
	"image/jpeg"
	"image/png"
	"net/http"
	"net/url"

	"github.com/nfnt/resize"
)

type scrapbookSitemap struct {
	Pages  []scrapbookPage
	Styles []scrapbookStyle
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

	logMessage(4, fmt.Sprintf("%s: %s", request.RemoteAddr, request.RequestURI))

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
			logMessage(2, err.Error())
			response.WriteHeader(500)
			fmt.Fprint(response, "Error.")
			return
		}

		_, err = db.Exec("INSERT INTO scrapbook_data.editors(session_id) VALUES($1)", token)
		if err != nil {
			logMessage(2, err.Error())
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

	// Edit API save
	if request.URL.Path == "/editapi/save" && request.Method == "POST" {
		var sitemap scrapbookSitemap

		if !isSessionAuthenticated(response, request) {
			return
		}

		err := json.NewDecoder(request.Body).Decode(&sitemap)
		if err != nil {
			logMessage(2, err.Error())
			response.WriteHeader(500)
			fmt.Fprintf(response, "Error.")
		} else {
			updateFromSitemap(sitemap)
			fmt.Fprintf(response, "Ok.")
		}
		return
	}

	// Edit api upload
	if request.URL.Path == "/editapi/upload" && request.Method == "POST" {
		if !isSessionAuthenticated(response, request) {
			return
		}

		err := request.ParseMultipartForm(6 << 20) //Max 6Mb
		if err != nil {
			response.WriteHeader(400)
			fmt.Fprintf(response, "Error.")
			return
		}

		file, handler, err := request.FormFile("upload")
		if err != nil {
			response.WriteHeader(400)
			fmt.Fprintf(response, "Error.")
			return
		}
		defer file.Close()

		logMessage(5, fmt.Sprintf("User uploaded file %s: %d %s", handler.Filename, handler.Size, handler.Header["Content-Type"]))

		if handler.Header["Content-Type"][0] != "image/png" && handler.Header["Content-Type"][0] != "image/jpeg" {
			response.WriteHeader(400)
			fmt.Fprintf(response, "Error.")
			return
		}

		var imageFile image.Image

		if handler.Header["Content-Type"][0] == "image/jpeg" {
			imageFile, err = jpeg.Decode(file)
		} else if handler.Header["Content-Type"][0] == "image/png" {
			imageFile, err = png.Decode(file)
		}
		if err != nil {
			response.WriteHeader(500)
			fmt.Fprintf(response, "Error.")
			return
		}

		mediaID, err := createMediaID()
		if err != nil {
			response.WriteHeader(500)
			fmt.Fprintf(response, "Error.")
			return
		}

		_, err = db.Exec("INSERT INTO scrapbook_data.media(media_id, media_type, media_name) VALUES ($1, $2, $3)", mediaID, "image", mediaID)
		if err != nil {
			response.WriteHeader(500)
			fmt.Fprintf(response, "Error.")
			return
		}

		// Generate optimised media
		for _, resolution := range imageResolutionSteps {
			if resolution <= imageFile.Bounds().Max.Y {
				logMessage(5, fmt.Sprintf("Encoding %vp image variant", resolution))
				mediaVersionID, err := createMediaVersionID()
				if err != nil {
					response.WriteHeader(500)
					fmt.Fprintf(response, "Error.")
					return
				}

				var options = jpeg.Options{
					Quality: 70,
				}

				resizedImage := resize.Resize(0, uint(resolution), imageFile, resize.Lanczos3)
				imageBuffer := new(bytes.Buffer)
				err = jpeg.Encode(imageBuffer, resizedImage, &options)
				if err != nil {
					response.WriteHeader(500)
					fmt.Fprintf(response, "Error.")
					return
				}
				imageBytes := imageBuffer.Bytes()

				_, err = db.Exec("INSERT INTO scrapbook_data.media_versions(media_version_id, media_id, version_width, version_height, media_data) VALUES ($1, $2, $3, $4, $5)", mediaVersionID, mediaID, resizedImage.Bounds().Max.X, resizedImage.Bounds().Max.Y, imageBytes)
				if err != nil {
					response.WriteHeader(500)
					fmt.Fprintf(response, "Error.")
					return
				}
			}
		}

		response.WriteHeader(200)
		fmt.Fprintf(response, "Ok.")
		return
	}

	// Serve sitemap
	if request.URL.Path == "/sitemap.json" {
		var (
			pages  []scrapbookPage  = []scrapbookPage{}
			styles []scrapbookStyle = []scrapbookStyle{}
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

		pageRows.Close()

		styleRows, err := db.Query("SELECT style_id, style_name, background_type, background_data, background_position, background_size, font_family, font_size, font_weight, font_color, margin, padding, text_align, border_width, border_style, border_color, custom_css FROM scrapbook_data.styles")
		if err != nil {
			logMessage(2, err.Error())
			return
		}

		for styleRows.Next() {
			var id, name, background_type, background_data, background_position, background_size, font_family, font_color, text_align, font_weight, border_style, border_color, custom_css string
			var border_width int
			var font_size, margin, padding float32
			styleRows.Scan(&id, &name, &background_type, &background_data, &background_position, &background_size, &font_family, &font_size, &font_weight, &font_color, &margin, &padding, &text_align, &border_width, &border_style, &border_color, &custom_css)

			styles = append(styles, scrapbookStyle{
				id, name, background_type, background_data, background_position, background_size, font_family, font_size, font_weight, font_color, margin, padding, text_align, border_width, border_style, border_color, custom_css,
			})
		}

		styleRows.Close()

		jsonBytes, err := json.Marshal(scrapbookSitemap{
			pages,
			styles,
		})

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

	formTemplate, err = template.ParseFiles("scrapbook.html")
	err = formTemplate.Execute(response, page)
	if err != nil {
		logMessage(1, err.Error())
		fmt.Fprintf(response, "Error.")
	}

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

func createMediaID() (string, error) {
	newToken := generateRandomString(8)
	err := db.QueryRow("SELECT media_id FROM scrapbook_data.media WHERE media_id = $1", newToken).Scan()
	if err == nil {
		return createMediaID()
	} else if err == sql.ErrNoRows {
		return newToken, nil
	}
	return "", err
}

func createMediaVersionID() (string, error) {
	newToken := generateRandomString(8)
	err := db.QueryRow("SELECT media_version_id FROM scrapbook_data.media_versions WHERE media_version_id = $1", newToken).Scan()
	if err == nil {
		return createMediaVersionID()
	} else if err == sql.ErrNoRows {
		return newToken, nil
	}
	return "", err
}

func getNestedElements(parentType string, parentId string) []scrapbookElement {
	elementRows, err := db.Query("SELECT element_id, element_name, style_id, pos_anchor, pos_x, pos_y, pos_z, width, height, is_link, link_url, text_content FROM scrapbook_data.elements WHERE parent_type = $1 AND parent_id = $2 ORDER BY sequence_number ASC", parentType, parentId)
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
		logMessage(5, content)
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

func updateFromSitemap(sitemap scrapbookSitemap) error {
	// Update styles

	_, err := db.Exec("DELETE FROM scrapbook_data.styles")
	if err != nil {
		logMessage(2, err.Error())
		return err
	}
	for _, style := range sitemap.Styles {
		logMessage(5, fmt.Sprintf("Processing style %s: %s", style.ID, style.Name))
		_, err := db.Exec("INSERT INTO scrapbook_data.styles(style_id, style_name, background_type, background_data, background_position, background_size, font_family, font_size, font_weight, font_color, margin, padding, text_align, border_width, border_style, border_color, custom_css) VALUES($1, $2, $3 ,$4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17)", style.ID, style.Name, style.BackgroundType, style.BackgroundData, style.BackgroundPosition, style.BackgroundSize, style.FontFamily, style.FontSize, style.FontWeight, style.FontColor, style.Margin, style.Padding, style.TextAlign, style.BorderWidth, style.BorderStyle, style.BorderColor, style.CustomCSS)
		if err != nil {
			logMessage(2, err.Error())
			return err
		}
	}

	// Update pages
	_, err = db.Exec("DELETE FROM scrapbook_data.pages")
	if err != nil {
		logMessage(2, err.Error())
		return err
	}
	_, err = db.Exec("DELETE FROM scrapbook_data.elements")
	if err != nil {
		logMessage(2, err.Error())
		return err
	}

	for _, page := range sitemap.Pages {
		logMessage(5, fmt.Sprintf("Processing page %s", page.Header.URI))
		_, err = db.Exec("INSERT INTO scrapbook_data.pages(page_uri, page_title) VALUES($1, $2)", page.Header.URI, page.Header.Title)
		if err != nil {
			logMessage(2, err.Error())
			return err
		}
		for i, element := range page.Elements {
			updateFromElement(element, "page", page.Header.URI, i)
		}
	}

	return nil
}

func updateFromElement(element scrapbookElement, parentType string, parentID string, sequenceNumber int) error {
	logMessage(5, fmt.Sprintf("Processing element %s: %s", element.ID, element.Name))
	logMessage(5, element.Content)
	_, err := db.Exec("INSERT INTO scrapbook_data.elements(element_id, parent_type, parent_id, sequence_number, element_name, style_id, pos_anchor, pos_x, pos_y, pos_z, width, height, is_link, link_url, text_content) VALUES ($1, $2, $3 ,$4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)", element.ID, parentType, parentID, sequenceNumber, element.Name, element.StyleID, element.PosAnchor, element.PosX, element.PosY, element.PosZ, element.Width, element.Height, parseBoolToInt(element.IsLink), element.LinkURL, element.Content)
	if err != nil {
		logMessage(2, err.Error())
		return err
	}

	for i, child := range element.Children {
		updateFromElement(child, "element", element.ID, i)
	}
	return nil
}

func parseBoolToInt(value bool) int {
	if value {
		return 1
	} else {
		return 0
	}
}
