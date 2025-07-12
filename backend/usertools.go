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
}

func getUserFromSession(r *http.Request, db *sql.DB) (*User, error) {
	userID, err := hasSession(r, db)
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

func hasSession(r *http.Request, db *sql.DB) (int, error) {
	cookie, err := r.Cookie("session_id")
	if err != nil {
		return 0, fmt.Errorf("no session cookie")
	}

	var userID int
	err = db.QueryRow(
		`SELECT user_id FROM sessions 
		 WHERE session_id = ? AND end_time > datetime('now')`,
		cookie.Value,
	).Scan(&userID)

	if err != nil {
		return 0, fmt.Errorf("invalid or expired session")
	}

	return userID, nil
}
