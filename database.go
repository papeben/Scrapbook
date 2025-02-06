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
		if err != nil {
			return err
		}
	} else if err != nil && n <= 1 {
		return err
	} else {
		logMessage(5, "Schema created")
	}

	err = db.QueryRow("SELECT version FROM configuration").Scan()
	if err == nil {
		return err
	}

	logMessage(5, "Creating tables")
	_, err = db.Exec("CREATE TABLE IF NOT EXISTS scrapbook_data.configuration (item VARCHAR(16) NOT NULL, value VARCHAR(16) NOT NULL, favicon BYTEA NULL)")
	if err != nil {
		return err
	}

	_, err = db.Exec("CREATE TABLE IF NOT EXISTS scrapbook_data.pages (page_uri VARCHAR(128) NOT NULL, page_title VARCHAR(128) NOT NULL DEFAULT 'New Page')")
	if err != nil {
		return err
	}

	_, err = db.Exec("CREATE TABLE IF NOT EXISTS scrapbook_data.element (element_id VARCHAR(8) NOT NULL, element_name VARCHAR(128) NOT NULL DEFAULT 'New Element',  style_id VARCHAR(8) NOT NULL DEFAULT 'DEFAULTS', position_anchor VARCHAR(16) NULL, position_x INT NOT NULL DEFAULT '0', position_y INT NOT NULL DEFAULT '0', position_z INT NOT NULL DEFAULT '0', is_link SMALLINT NOT NULL DEFAULT '0' , link_url VARCHAR(128) NULL)")
	if err != nil {
		return err
	}

	_, err = db.Exec("CREATE TABLE IF NOT EXISTS scrapbook_data.element_parent (element_id VARCHAR(8) NOT NULL, parent_type VARCHAR(8) NOT NULL DEFAULT 'page', parent_id VARCHAR(8) NULL, sequence_number INT NOT NULL DEFAULT '1')")
	if err != nil {
		return err
	}

	_, err = db.Exec("CREATE TABLE IF NOT EXISTS scrapbook_data.styles (style_id VARCHAR(8) NOT NULL, style_name VARCHAR(128) NOT NULL DEFAULT 'new_style', background_type VARCHAR(8) NOT NULL DEFAULT 'color', background_data VARCHAR(128) NOT NULL DEFAULT '#cccccc', background_position VARCHAR(16) NOT NULL DEFAULT 'center', background_size VARCHAR(16) NOT NULL DEFAULT 'cover', font_family VARCHAR(32) NOT NULL DEFAULT 'sans-serif', font_size INT NOT NULL DEFAULT '16', font_weight VARCHAR(16) NOT NULL DEFAULT 'normal', font_color VARCHAR(16) NOT NULL DEFAULT '#000000', margin INT NOT NULL DEFAULT '0', padding INT NOT NULL DEFAULT '0', text_align VARCHAR(16) NOT NULL DEFAULT 'left', border_width INT NOT NULL DEFAULT '0', border_style VARCHAR(16) NOT NULL DEFAULT 'solid', border_color VARCHAR(32) NOT NULL DEFAULT '#000000', custom_css TEXT NULL)")
	if err != nil {
		return err
	}

	_, err = db.Exec("CREATE TABLE IF NOT EXISTS scrapbook_data.media (media_id VARCHAR(8) NOT NULL, media_type VARCHAR(8) NOT NULL DEFAULT 'image', media_height INT NOT NULL, media_width INT NOT NULL, media_data BYTEA)")
	if err != nil {
		return err
	}

	return nil
}
