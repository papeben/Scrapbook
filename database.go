package main

import (
	"database/sql"
	"fmt"
	"time"
)

func dbConnect(n int) error {
	var err error
	db, err = sql.Open("postgres", fmt.Sprintf("postgres://%s:%s@%s/%s?sslmode=%s", POSTGRES_USER, POSTGRES_PASS, POSTGRES_HOST, POSTGRES_DB, POSTGRES_SSL))
	if err != nil {
		return err
	}

	_, err = db.Exec("CREATE SCHEMA IF NOT EXISTS scrapbook_data")
	if err != nil && n > 1 {
		logMessage(2, fmt.Sprintf("Failed to create database connection: %s", err.Error()))
		logMessage(2, "Retrying connection in 5 seconds...")
		time.Sleep(5 * time.Second)
		err = dbConnect(n - 1)
		return err
	} else if err != nil && n <= 1 {
		return err
	} else {
		logMessage(5, "PostgreSQL connection established")
	}

	var dbVersion string
	err = db.QueryRow("SELECT value FROM scrapbook_data.configuration WHERE item = 'version'").Scan(&dbVersion)
	if err == nil {
		logMessage(4, "Scrapbook schema present. Continuing startup...")
		return nil
	} else {
		logMessage(4, "Scrapbook schema not present. Installing...")
	}

	logMessage(5, "Creating tables")
	_, err = db.Exec("CREATE TABLE IF NOT EXISTS scrapbook_data.configuration (item VARCHAR(16) NOT NULL PRIMARY KEY, value TEXT NOT NULL)")
	if err != nil {
		return err
	}

	_, err = db.Exec("CREATE TABLE IF NOT EXISTS scrapbook_data.pages (page_uri VARCHAR(128) NOT NULL PRIMARY KEY, page_title VARCHAR(128) NOT NULL DEFAULT 'New Page')")
	if err != nil {
		return err
	}

	_, err = db.Exec("CREATE TABLE IF NOT EXISTS scrapbook_data.elements (element_id VARCHAR(8) NOT NULL PRIMARY KEY, parent_type VARCHAR(8) NOT NULL DEFAULT 'page', parent_id VARCHAR(128) NULL, sequence_number INT NOT NULL DEFAULT '1', element_name VARCHAR(128) NOT NULL DEFAULT 'New Element',  style_id VARCHAR(8) NOT NULL DEFAULT 'DEFAULTS', pos_anchor VARCHAR(16) NULL, pos_x REAL NOT NULL DEFAULT '0', pos_y REAL NOT NULL DEFAULT '0', pos_z INT NOT NULL DEFAULT '0', width INT NOT NULL DEFAULT 10, height INT NOT NULL DEFAULT 10, is_link SMALLINT NOT NULL DEFAULT '0' , link_url VARCHAR(128) NULL, text_content TEXT NULL)")
	if err != nil {
		return err
	}

	_, err = db.Exec("CREATE TABLE IF NOT EXISTS scrapbook_data.styles (style_id VARCHAR(8) NOT NULL PRIMARY KEY, style_name VARCHAR(128) NOT NULL DEFAULT 'new_style', background_type VARCHAR(8) NOT NULL DEFAULT 'color', background_data VARCHAR(128) NOT NULL DEFAULT '#cccccc', background_position VARCHAR(16) NOT NULL DEFAULT 'center', background_size VARCHAR(16) NOT NULL DEFAULT 'cover', font_family VARCHAR(32) NOT NULL DEFAULT 'sans-serif', font_size REAL NOT NULL DEFAULT '16', font_weight VARCHAR(16) NOT NULL DEFAULT 'normal', font_color VARCHAR(16) NOT NULL DEFAULT '#000000', margin REAL NOT NULL DEFAULT '0', padding REAL NOT NULL DEFAULT '0', text_align VARCHAR(16) NOT NULL DEFAULT 'left', border_width INT NOT NULL DEFAULT '0', border_style VARCHAR(16) NOT NULL DEFAULT 'solid', border_color VARCHAR(32) NOT NULL DEFAULT '#000000', custom_css TEXT NULL)")
	if err != nil {
		return err
	}

	_, err = db.Exec("CREATE TABLE IF NOT EXISTS scrapbook_data.media (media_id VARCHAR(8) NOT NULL PRIMARY KEY, media_type VARCHAR(8) NOT NULL DEFAULT 'image', media_name TEXT NULL)")
	if err != nil {
		return err
	}

	_, err = db.Exec("CREATE TABLE IF NOT EXISTS scrapbook_data.media_versions (media_version_id VARCHAR(8) NOT NULL PRIMARY KEY, media_id VARCHAR(8) NOT NULL, version_width INT NOT NULL, version_height INT NOT NULL, media_data BYTEA)")
	if err != nil {
		return err
	}

	_, err = db.Exec("CREATE TABLE IF NOT EXISTS scrapbook_data.editors (session_id VARCHAR(256) NOT NULL PRIMARY KEY, timestamp TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP)")
	if err != nil {
		return err
	}

	// Insert initial data
	_, err = db.Exec("INSERT INTO scrapbook_data.configuration(item, value) VALUES('version', '1')")
	if err != nil {
		return err
	}

	_, err = db.Exec("INSERT INTO scrapbook_data.pages(page_uri, page_title) VALUES('/', 'Homepage')")
	if err != nil {
		return err
	}

	_, err = db.Exec("INSERT INTO scrapbook_data.elements(element_id, parent_type, parent_id, sequence_number, element_name, style_id, pos_anchor, pos_x, pos_y, pos_z, width, height, is_link, link_url, text_content) VALUES('AAAAAAAA', 'page', '/', 0, 'Default Element', 'AAAAAAAA', 'none', '0', '0', '0', '200', '200', 0, '', 'Hello world')")
	if err != nil {
		return err
	}

	_, err = db.Exec("INSERT INTO scrapbook_data.elements(element_id, parent_type, parent_id, sequence_number, element_name, style_id, pos_anchor, pos_x, pos_y, pos_z, width, height, is_link, link_url, text_content) VALUES('BBBBBBBB', 'element', 'AAAAAAAA', 0, 'Default Nested Element', 'BBBBBBBB', 'none', '0', '0', '0', '20', '20', 0, '', 'Hello parent!')")
	if err != nil {
		return err
	}

	_, err = db.Exec("INSERT INTO scrapbook_data.styles(style_id, style_name, background_type, background_data, background_position, background_size, font_family, font_size, font_weight, font_color, margin, padding, text_align, border_width, border_style, border_color, custom_css) VALUES('AAAAAAAA', 'Default Style', 'color', '#123123', 'center', 'cover', 'sans-serif', '10', 'normal', '#000000', '10', '10', 'left', '1', 'solid', '#ff0000', 'font-weight: bold;')")
	if err != nil {
		return err
	}

	_, err = db.Exec("INSERT INTO scrapbook_data.styles(style_id, style_name, background_type, background_data, background_position, background_size, font_family, font_size, font_weight, font_color, margin, padding, text_align, border_width, border_style, border_color, custom_css) VALUES('BBBBBBBB', 'Default Child Style', 'color', '#420420', 'center', 'cover', 'sans-serif', '2', 'normal', '#000000', '10', '10', 'left', '1', 'solid', '#ff0000', 'font-weight: bold;')")
	if err != nil {
		return err
	}

	return nil
}
