package main

import (
	"database/sql"
	"fmt"
	"net/http"
)

type User struct {
	ID    int
	Email string
}

type TemplateData struct {
	IsAuthenticated bool
	User            *User
	Data            map[string]interface{}
}

type Post struct {
	Title   string
	Date    string
	Summary string
	Topics  []string
}

func getUserFromSession(r *http.Request, db *sql.DB) (*User, error) {
	userID, _, err := hasSession(r, db)
	if err != nil {
		return nil, err
	}

	fmt.Printf("User ID: %d\n", userID)

	var user User
	err = db.QueryRow(
		"SELECT user_id, email FROM users WHERE user_id = ?",
		userID,
	).Scan(&user.ID, &user.Email)

	if err != nil {
		return nil, fmt.Errorf("user not found: %s", err)
	}

	return &user, nil
}

func hasSession(r *http.Request, db *sql.DB) (int, string, error) {
	cookie, err := r.Cookie("session_id")
	if err != nil {
		return -1, "", fmt.Errorf("no session cookie")
	}

	var userID int
	err = db.QueryRow(
		`SELECT user_id FROM sessions 
		 WHERE session_id = ? AND end_time > datetime('now')`,
		cookie.Value,
	).Scan(&userID)

	if err != nil {
		return 0, "", fmt.Errorf("invalid or expired session")
	}

	return userID, cookie.Value, nil
}

func getPosts(db *sql.DB) (*[]Post, error) {
	var posts []Post

	rows, err := db.Query(
		`SELECT p.post_id, p.title, p.created_at, t.tag_name
		FROM (
			SELECT * FROM posts WHERE public = 1 ORDER BY created_at DESC LIMIT 5
		) p
		JOIN posts_tags pt ON pt.post_id = p.post_id
		JOIN tags t ON t.tag_id = pt.tag_id;
		`,
	)

	if err != nil {
		return nil, fmt.Errorf("db error: %s", err)
	}
	defer rows.Close()

	return &posts, nil
}
