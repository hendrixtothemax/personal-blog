package main

import (
	"database/sql"
	"log"
)

func startDB() *sql.DB {
	db, err := sql.Open("sqlite3", "./main.db")
	if err != nil {
		log.Fatal(err)
	}

	const createTable = `
	CREATE TABLE IF NOT EXISTS users (
        user_id INTEGER PRIMARY KEY AUTOINCREMENT,
        email TEXT NOT NULL UNIQUE,
        password TEXT NOT NULL,
		created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
		updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
		last_login TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
    );
	`

	const createSessionTable = `
	CREATE TABLE IF NOT EXISTS sessions (
		session_id TEXT NOT NULL UNIQUE,
		user_id INTEGER NOT NULL,
		created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
		last_use TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
		end_time TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now', '+24 hours')),
		FOREIGN KEY (user_id) REFERENCES users(user_id) ON DELETE CASCADE
	);
	`

	const postTable = `
	PRAGMA foreign_keys = ON;

	CREATE TABLE IF NOT EXISTS posts (
		post_id INTEGER PRIMARY KEY AUTOINCREMENT,
		title TEXT NOT NULL,
		file_loc TEXT NOT NULL,
		public BOOLEAN DEFAULT TRUE,
		created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
	);
	`

	const tagsTable = `
	CREATE TABLE IF NOT EXISTS tags (
		tag_id INTEGER PRIMARY KEY AUTOINCREMENT,
		tag_name TEXT UNIQUE NOT NULL,
		created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
	);
	`

	const postTagsTable = `
	CREATE TABLE IF NOT EXISTS posts_tags (
		post_id INTEGER NOT NULL,
		tag_id INTEGER UNIQUE NOT NULL,
		created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
		FOREIGN KEY (post_id) REFERENCES posts(post_id) ON DELETE CASCADE,
		FOREIGN KEY (tag_id) REFERENCES tags(tag_id) ON DELETE CASCADE,
		PRIMARY KEY (post_id, tag_id)
	);
	`

	const cleanupSessions = `
	DELETE FROM sessions 
	WHERE end_time < strftime('%Y-%m-%dT%H:%M:%SZ', 'now')
	`

	_, err = db.Exec("PRAGMA foreign_keys = ON;")
	if err != nil {
		log.Fatal(err)
	}

	_, err = db.Exec(createTable)
	if err != nil {
		log.Fatal(err)
	}

	_, err = db.Exec(createSessionTable)
	if err != nil {
		log.Fatal(err)
	}

	_, err = db.Exec(postTable)
	if err != nil {
		log.Fatal(err)
	}

	_, err = db.Exec(tagsTable)
	if err != nil {
		log.Fatal(err)
	}

	_, err = db.Exec(postTagsTable)
	if err != nil {
		log.Fatal(err)
	}

	_, err = db.Exec(cleanupSessions)
	if err != nil {
		log.Fatal(err)
	}

	return db
}
