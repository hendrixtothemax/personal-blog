package main

import (
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"fmt"
	"log"
	"net/http"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

func registerUser(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Cache-Control", "no-cache")
		w.Header().Add("Content-Type", "text/html; charset=utf-8")

		if r.Method != http.MethodPost {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			return
		}

		if err := r.ParseForm(); err != nil {
			http.Error(w, "Unable to parse form", http.StatusBadRequest)
			return
		}

		email := r.FormValue("email")
		password := r.FormValue("password")

		fmt.Println("email:", email, "| password:", password)

		// Here, you can use db to run queries, e.g. insert user
		exists, err := emailExists(db, email)
		if err != nil {
			http.Error(w, "Database error", http.StatusInternalServerError)
			return
		}
		if exists {
			http.Error(w, "Email already registered", http.StatusConflict)
			return
		}

		password = strings.TrimSpace(password)

		hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)

		if err != nil {
			http.Error(w, "Hashing Error", http.StatusInternalServerError)
			return
		}

		result, err := db.Exec("INSERT INTO users (email, password) VALUES (?, ?)", email, hash)
		if err != nil {
			http.Error(w, "Database error", http.StatusInternalServerError)
			return
		}

		user_id, err := result.LastInsertId()
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println("New user ID:", user_id)

		session_id, err := generateSession(db, user_id)

		if err != nil {
			http.Error(w, "Failed to create session", http.StatusInternalServerError)
			return
		}

		http.SetCookie(w, &http.Cookie{
			Name:     "session_id",
			Value:    session_id,
			HttpOnly: true,
			Secure:   false, // only if using HTTPS
			Path:     "/",
			SameSite: http.SameSiteStrictMode,
		})

		w.Header().Set("HX-Redirect", "/testmd")
	}
}

func generateSession(db *sql.DB, userid int64) (string, error) {
	session_id, err := generateSessionID()
	if err != nil {
		return "", err
	}

	//strftime('%Y-%m-%dT%H:%M:%SZ', 'now', '+24 hours'
	_, err = db.Exec("INSERT INTO sessions (session_id, user_id, end_time) VALUES (?, ?, strftime('%Y-%m-%dT%H:%M:%SZ', 'now', '+48 hours'))", session_id, userid)
	if err != nil {
		return "", err
	}

	return session_id, nil
}

func generateSessionID() (string, error) {
	b := make([]byte, 32) // 256-bit token
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

func loginUser(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Cache-Control", "no-cache")
		w.Header().Add("Content-Type", "text/html; charset=utf-8")

		if r.Method != http.MethodPost {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			return
		}

		if err := r.ParseForm(); err != nil {
			http.Error(w, "Unable to parse form", http.StatusBadRequest)
			return
		}

		email := r.FormValue("email")
		password := r.FormValue("password")

		fmt.Println("email:", email, "| password:", password)

		// Here, you can use db to run queries, e.g. insert user
		exists, err := emailExists(db, email)
		if err != nil {
			http.Error(w, "Database error", http.StatusInternalServerError)
			return
		}
		if !exists {
			http.Error(w, "Email is not registered", http.StatusNotFound)
			return
		}

		var hashedPassword string
		var user_id_db int64

		err = db.QueryRow("SELECT password, user_id FROM users WHERE email = ? LIMIT 1", email).Scan(&hashedPassword, &user_id_db)

		if err == sql.ErrNoRows {
			http.Error(w, "Invalid email or password", http.StatusUnauthorized)
			return
		} else if err != nil {
			log.Printf("DB error: %v", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		hashedPassword = strings.TrimSpace(hashedPassword)
		password = strings.TrimSpace(password)

		err = bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password))
		if err != nil {
			log.Println(err)
			log.Println("Password does not match")
			http.Error(w, "Password does not match", http.StatusUnauthorized)
			return
		}

		log.Println("Password is correct")
		session_id, err := generateSession(db, user_id_db)

		if err != nil {
			http.Error(w, "Failed to create session", http.StatusInternalServerError)
			return
		}

		_, err2 := db.Exec("UPDATE users SET last_login = strftime('%Y-%m-%dT%H:%M:%SZ', 'now') WHERE user_id = ?", user_id_db)
		if err2 != nil {
			log.Printf("Failed to update last_login: %v\n", err2)
		}

		http.SetCookie(w, &http.Cookie{
			Name:     "session_id",
			Value:    session_id,
			HttpOnly: true,
			Secure:   false, // only if using HTTPS
			Path:     "/",
			SameSite: http.SameSiteStrictMode,
		})

		w.Header().Set("HX-Redirect", "/testmd")
	}
}

func emailExists(db *sql.DB, email string) (bool, error) {
	var count int
	err := db.QueryRow("SELECT COUNT(1) FROM users WHERE email = ?", email).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}
