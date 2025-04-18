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
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/nfnt/resize"
)

type scrapbookSitemap struct {
	Pages  []scrapbookPage
	Styles []scrapbookStyle
	Media  []scrapbookMedia
	Fonts  []scrapbookFont
}

type scrapbookPage struct {
	Header   scrapbookPageHeader
	Elements []scrapbookElement
}

type scrapbookElement struct {
	ID          string
	StyleID     string
	Width       string
	Height      string
	IsLink      bool
	LinkURL     string
	ContentType string
	Content     string
	Direction   string
	Wrap        string
	Justify     string
	Children    []scrapbookElement
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
	Title           string
	URI             string
	Description     string
	HasPreviewImage bool
	PreviewImage    string
}

type scrapbookMedia struct {
	MediaID       string
	MediaType     string
	MediaName     string
	MediaVersions []scrapbookMediaVersion
}

type scrapbookMediaVersion struct {
	VID    string
	Width  int
	Height int
}

type scrapbookFont struct {
	FontID   string
	FontName string
}

func httpHandler(response http.ResponseWriter, request *http.Request) {

	logMessage(4, fmt.Sprintf("%s: %s", request.RemoteAddr, request.RequestURI))

	if request.URL.Path == "/editapi/requestedit" { // Edit API
		handleRequestEdit(response, request)
	} else if request.URL.Path == "/editapi/save" && request.Method == "POST" { // Edit API save
		handleSave(response, request)
	} else if request.URL.Path == "/editapi/upload" && request.Method == "POST" { // Edit api upload
		handleMediaUpload(response, request)
	} else if strings.HasPrefix(request.URL.Path, MEDIA_DIRECTORY) { // Serve media
		handleServeMedia(response, request)
	} else if strings.HasPrefix(request.URL.Path, FONT_DIRECTORY) { // Serve fonts
		handleServeFont(response, request)
	} else if request.URL.Path == "/sitemap.json" { // Serve sitemap
		handleServeSitemap(response, request)
	} else { // Serve page (or 404 if no page)
		var (
			title           string
			description     string
			hasPreviewImage bool = false
			previewImage    string
			page            scrapbookPageHeader
		)
		err := db.QueryRow("SELECT page_title, page_description, preview_image FROM scrapbook_data.pages WHERE page_uri = $1", request.URL.Path).Scan(&title, &description, &previewImage) // Get page info from database
		if err == sql.ErrNoRows {
			page = scrapbookPageHeader{
				"Page Not Found",
				request.URL.Path,
				"This page does not exist.",
				false,
				"",
			}
			response.WriteHeader(404)
		} else if err != sql.ErrNoRows && err != nil {
			logMessage(2, err.Error())
		} else {
			if previewImage != "" {
				hasPreviewImage = true
			}

			page = scrapbookPageHeader{
				title,
				request.URL.Path,
				description,
				hasPreviewImage,
				previewImage,
			}
		}

		formTemplate, err = template.ParseFiles("editor.html") // TODO REMOVE LINE
		err = formTemplate.Execute(response, page)
		if err != nil {
			logMessage(1, err.Error())
			fmt.Fprintf(response, "Error.")
		}
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

func createFontID() (string, error) {
	newToken := generateRandomString(8)
	err := db.QueryRow("SELECT font_id FROM scrapbook_data.fonts WHERE font_id = $1", newToken).Scan()
	if err == nil {
		return createFontID()
	} else if err == sql.ErrNoRows {
		return newToken, nil
	}
	return "", err
}

func getNestedElements(parentType string, parentId string) []scrapbookElement {
	elementRows, err := db.Query("SELECT element_id, style_id, width, height, is_link, link_url, content_type, content, direction, wrap, justify FROM scrapbook_data.elements WHERE parent_type = $1 AND parent_id = $2 ORDER BY sequence_number ASC", parentType, parentId)
	if err != nil {
		logMessage(2, err.Error())
	}

	var (
		elements     []scrapbookElement = []scrapbookElement{}
		element_id   string
		style_id     string
		width        string
		height       string
		is_link      bool
		link_url     string
		content_type string
		content      string
		direction    string
		wrap         string
		justify      string
	)

	for elementRows.Next() {
		elementRows.Scan(&element_id, &style_id, &width, &height, &is_link, &link_url, &content_type, &content, &direction, &wrap, &justify)
		elements = append(elements, scrapbookElement{
			element_id,
			style_id,
			width,
			height,
			is_link,
			link_url,
			content_type,
			content,
			direction,
			wrap,
			justify,
			getNestedElements("element", element_id),
		})
	}
	return elements
}

func updateFromSitemap(sitemap scrapbookSitemap) error {
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
		_, err = db.Exec("INSERT INTO scrapbook_data.pages(page_uri, page_title, page_description, preview_image) VALUES($1, $2, $3, $4)", page.Header.URI, page.Header.Title, page.Header.Description, page.Header.PreviewImage)
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
	logMessage(5, fmt.Sprintf("Processing element %s", element.ID))
	logMessage(5, element.Content)
	_, err := db.Exec("INSERT INTO scrapbook_data.elements(element_id, parent_type, parent_id, sequence_number, style_id, width, height, is_link, link_url, content_type, content, direction, wrap, justify) VALUES ($1, $2, $3 ,$4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)", element.ID, parentType, parentID, sequenceNumber, element.StyleID, element.Width, element.Height, parseBoolToInt(element.IsLink), element.LinkURL, element.ContentType, element.Content, element.Direction, element.Wrap, element.Justify)
	if err != nil {
		logMessage(2, err.Error())
		return err
	}

	for i, child := range element.Children {
		updateFromElement(child, "element", element.ID, i)
	}
	return nil
}

func handleMediaUpload(response http.ResponseWriter, request *http.Request) {
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

	if handler.Header["Content-Type"][0] == "image/png" || handler.Header["Content-Type"][0] == "image/jpeg" {
		handleImageUpload(response, handler, file)
	} else if handler.Header["Content-Type"][0] == "video/mp4" || handler.Header["Content-Type"][0] == "video/x-matroska" {
		handleVideoUpload(response, handler, file)
	} else if handler.Header["Content-Type"][0] == "font/ttf" {
		handleFontUpload(response, handler, file)
	} else {
		response.WriteHeader(400)
		fmt.Fprintf(response, "Error.")
	}
}

func handleImageUpload(response http.ResponseWriter, handler *multipart.FileHeader, file multipart.File) {

	var imageFile image.Image
	var err error

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

	_, err = db.Exec("INSERT INTO scrapbook_data.media(media_id, media_type, media_name) VALUES ($1, $2, $3)", mediaID, "image", handler.Filename)
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
}

func handleVideoUpload(response http.ResponseWriter, handler *multipart.FileHeader, file multipart.File) {
	mediaID, err := createMediaID()
	if err != nil {
		response.WriteHeader(500)
		fmt.Fprintf(response, "Error.")
		return
	}
	tempFilePath := filepath.Join(TEMP_DIRECTORY, mediaID)

	tempFile, err := os.Create(tempFilePath)
	if err != nil {
		response.WriteHeader(500)
		fmt.Fprintf(response, "Error.10")
		return
	}

	_, err = io.Copy(tempFile, file)
	if err != nil {
		response.WriteHeader(500)
		fmt.Fprintf(response, "Error.9")
		return
	}
	tempFile.Close()

	output, err := exec.Command("ffprobe", "-v", "error", "-select_streams", "v:0", "-show_entries", "stream=width,height", "-of", "csv=s=x:p=0", tempFilePath).CombinedOutput()
	if err != nil {
		errWithWeb(err, response, string(output))
		return
	}

	_, err = db.Exec("INSERT INTO scrapbook_data.media(media_id, media_type, media_name) VALUES ($1, $2, $3)", mediaID, "video", handler.Filename)
	if err != nil {
		response.WriteHeader(500)
		fmt.Fprintf(response, "Error.7")
		return
	}

	go renderOptimisedVideo(mediaID)

	response.WriteHeader(200)
	fmt.Fprintf(response, "Ok.")

}

func renderOptimisedVideo(mediaID string) {
	tempFilePath := filepath.Join(TEMP_DIRECTORY, mediaID)
	tempFilePathRender := filepath.Join(TEMP_DIRECTORY, fmt.Sprintf("%srender.%s", mediaID, videoFFMPEGContainer))

	output, err := exec.Command("ffprobe", "-v", "error", "-select_streams", "v:0", "-show_entries", "stream=width,height", "-of", "csv=s=x:p=0", tempFilePath).CombinedOutput()
	if err != nil {
		return
	}
	mediaHeight, err := strconv.Atoi(strings.Split(strings.TrimSpace(string(output)), "x")[1])
	if err != nil {
		return
	}

	// Generate optimised media
	for i, resolution := range imageResolutionSteps {
		if resolution <= mediaHeight {
			logMessage(5, fmt.Sprintf("Encoding %vp video variant", resolution))
			mediaVersionID, err := createMediaVersionID()
			if err != nil {
				return
			}

			output, err = exec.Command("ffmpeg", "-y", "-i", tempFilePath, "-c:v", videoFFMPEGCodec, "-b:v", fmt.Sprintf("%vM", videoBitrateSteps[i]), "-vf", fmt.Sprintf("scale=-2:%v", resolution), "-preset", videoFFMPEGPreset, "-c:a", videoFFMPEGAudioCodec, "-movflags", "+faststart", tempFilePathRender).CombinedOutput()
			logMessage(5, string(output))
			if err != nil {
				return
			}

			renderFile, err := os.Open(tempFilePathRender)
			if err != nil {
				return
			}
			videoBytes, err := io.ReadAll(renderFile)
			if err != nil {
				return
			}
			renderFile.Close()

			output, err := exec.Command("ffprobe", "-v", "error", "-select_streams", "v:0", "-show_entries", "stream=width,height", "-of", "csv=s=x:p=0", tempFilePathRender).CombinedOutput()
			if err != nil {
				return
			}

			newMediaHeight, err := strconv.Atoi(strings.Split(strings.TrimSpace(string(output)), "x")[1])
			if err != nil {
				return
			}

			newMediaWidth, err := strconv.Atoi(strings.Split(string(output), "x")[0])
			if err != nil {
				return
			}

			_, err = db.Exec("INSERT INTO scrapbook_data.media_versions(media_version_id, media_id, version_width, version_height, media_data) VALUES ($1, $2, $3, $4, $5)", mediaVersionID, mediaID, newMediaWidth, newMediaHeight, videoBytes)
			if err != nil {
				return
			}

			err = os.Remove(tempFilePathRender)
			if err != nil {
				return
			}
		}
	}

	err = os.Remove(tempFilePath)
	if err != nil {
		return
	}
}

func handleFontUpload(response http.ResponseWriter, handler *multipart.FileHeader, file multipart.File) {
	fontID, err := createFontID()
	if err != nil {
		errWithWeb(err, response, "Error creating font ID")
		return
	}

	fontBuffer := new(bytes.Buffer)
	n, err := fontBuffer.ReadFrom(file)
	logMessage(5, fmt.Sprintf("Read %v bytes from font file.", n))
	if err != nil {
		errWithWeb(err, response, "Error reading font file.")
		return
	}

	_, err = db.Exec("INSERT INTO scrapbook_data.fonts(font_id, font_name, font_bytes) VALUES ($1, $2, $3)", fontID, handler.Filename, fontBuffer.Bytes())
	if err != nil {
		errWithWeb(err, response, "Error adding font db record.")
		return
	}

	response.WriteHeader(200)
	fmt.Fprintf(response, "Ok.")
}

func handleRequestEdit(response http.ResponseWriter, request *http.Request) {
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
}

func handleSave(response http.ResponseWriter, request *http.Request) {
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
}

func handleServeMedia(response http.ResponseWriter, request *http.Request) {
	pathSplit := strings.Split(request.URL.Path, "/")
	mediaID := strings.Split(pathSplit[len(pathSplit)-1], ".")[0]
	var data []byte

	err := db.QueryRow("SELECT media_data FROM scrapbook_data.media_versions WHERE media_version_id = $1", mediaID).Scan(&data)
	if err == sql.ErrNoRows {
		response.WriteHeader(404)
		fmt.Fprint(response, `Not Found.`)
		return
	} else if err != nil {
		response.WriteHeader(500)
		fmt.Fprint(response, `Error.`)
		return
	}

	_, err = response.Write(data)
	if err != nil {
		response.WriteHeader(500)
		fmt.Fprint(response, `Error.`)
		return
	}
}

func handleServeFont(response http.ResponseWriter, request *http.Request) {
	pathSplit := strings.Split(request.URL.Path, "/")
	fontID := strings.Split(pathSplit[len(pathSplit)-1], ".")[0]
	var data []byte

	err := db.QueryRow("SELECT font_bytes FROM scrapbook_data.fonts WHERE font_id = $1", fontID).Scan(&data)
	if err == sql.ErrNoRows {
		response.WriteHeader(404)
		fmt.Fprint(response, `Not Found.`)
		return
	} else if err != nil {
		response.WriteHeader(500)
		fmt.Fprint(response, `Error.`)
		return
	}

	_, err = response.Write(data)
	if err != nil {
		response.WriteHeader(500)
		fmt.Fprint(response, `Error.`)
		return
	}
}

func handleServeSitemap(response http.ResponseWriter, request *http.Request) {
	var (
		pages  []scrapbookPage  = []scrapbookPage{}
		styles []scrapbookStyle = []scrapbookStyle{}
		media  []scrapbookMedia = []scrapbookMedia{}
		fonts  []scrapbookFont  = []scrapbookFont{}
	)

	pageRows, err := db.Query("SELECT page_title, page_uri, page_description, preview_image FROM scrapbook_data.pages")
	if err != nil {
		logMessage(2, err.Error())
		return
	}

	for pageRows.Next() {
		var (
			title           string
			description     string
			uri             string
			hasPreviewImage bool
			previewImage    string
		)
		pageRows.Scan(&title, &uri, &description, &previewImage)

		if previewImage != "" {
			hasPreviewImage = true
		}

		pages = append(pages, scrapbookPage{
			scrapbookPageHeader{
				title,
				uri,
				description,
				hasPreviewImage,
				previewImage,
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

	mediaRows, err := db.Query("SELECT media_id, media_type, media_name FROM scrapbook_data.media")
	if err != nil {
		logMessage(2, err.Error())
		return
	}
	for mediaRows.Next() {
		var mediaID, mediaType, mediaName string
		mediaRows.Scan(&mediaID, &mediaType, &mediaName)
		mediaVersionRows, err := db.Query("SELECT media_version_id, version_width, version_height FROM scrapbook_data.media_versions WHERE media_id = $1", mediaID)
		if err != nil {
			logMessage(2, err.Error())
			return
		}
		var mediaVersions = []scrapbookMediaVersion{}
		for mediaVersionRows.Next() {
			var (
				mediaVersionID string
				versionWidth   int
				versionHeight  int
			)
			mediaVersionRows.Scan(&mediaVersionID, &versionWidth, &versionHeight)
			mediaVersions = append(mediaVersions, scrapbookMediaVersion{
				mediaVersionID,
				versionWidth,
				versionHeight,
			})
		}
		media = append(media, scrapbookMedia{
			mediaID,
			mediaType,
			mediaName,
			mediaVersions,
		})
	}
	mediaRows.Close()

	fontRows, err := db.Query("SELECT font_id, font_name FROM scrapbook_data.fonts")
	if err != nil {
		logMessage(2, err.Error())
		return
	}
	for fontRows.Next() {
		var fontID, fontName string
		fontRows.Scan(&fontID, &fontName)

		fonts = append(fonts, scrapbookFont{
			fontID,
			fontName,
		})
	}
	mediaRows.Close()

	jsonBytes, err := json.Marshal(scrapbookSitemap{
		pages,
		styles,
		media,
		fonts,
	})

	if err != nil {
		logMessage(2, err.Error())
		return
	}

	fmt.Fprint(response, string(jsonBytes))
}
