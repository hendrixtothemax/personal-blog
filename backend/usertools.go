package main

import (
	"bytes"
	"database/sql"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	chromahtml "github.com/alecthomas/chroma/v2/formatters/html"
	"github.com/yuin/goldmark"
	highlighting "github.com/yuin/goldmark-highlighting/v2"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/renderer/html"
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
		`
		SELECT p.post_id, p.title, p.summary, p.created_at, p.file_loc, GROUP_CONCAT(t.tag_name) AS tags
    	FROM (
			SELECT post_id, title, summary, created_at, file_loc
			FROM posts
			WHERE public = 1
			ORDER BY created_at DESC
			LIMIT 5
		) AS p
		LEFT JOIN posts_tags pt ON pt.post_id = p.post_id
		LEFT JOIN tags t ON t.tag_id = pt.tag_id
		GROUP BY p.post_id, p.title, p.summary, p.created_at, p.file_loc
		ORDER BY p.created_at DESC;
		`,
	)

	if err != nil {
		return nil, fmt.Errorf("db error: %s", err)
	}
	defer rows.Close()

	numbRows := 0

	for rows.Next() {
		var post_id string
		var title string
		var summary string
		var created_at string
		var file_loc string
		var tags sql.NullString

		err := rows.Scan(&post_id, &title, &summary, &created_at, &file_loc, &tags)

		if err != nil {
			return nil, fmt.Errorf("db row error: %s", err)
		}

		tagList := []string{}
		if tags.Valid && tags.String != "" {
			tagList = strings.Split(tags.String, ",")
		}

		if len(tagList) < 1 {
			tagList = append(tagList, "None")
		}

		fmt.Printf("PID: %s | Title: %s | Created At: %s | File Loc: %s | Tag Numbs: %d\n", post_id, title, created_at, file_loc, len(tagList))

		curPost := Post{
			Title:   title,
			Summary: summary,
			Date:    created_at,
			Topics:  tagList,
		}

		posts = append(posts, curPost)

		numbRows += 1
	}

	fmt.Printf("Number of Rows: %d\n", numbRows)

	return &posts, nil
}

func mdToHTML(path string) (string, error) {
	file, err := os.Open("./posts/" + path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer

	md := goldmark.New(
		goldmark.WithExtensions(
			extension.GFM,
			highlighting.NewHighlighting(
				highlighting.WithStyle("nord"),
				highlighting.WithGuessLanguage(true),
				highlighting.WithFormatOptions(
					chromahtml.WithClasses(false),
				),
			),
		),
		goldmark.WithRendererOptions(
			html.WithHardWraps(),
			html.WithXHTML(),
			html.WithUnsafe(),
		),
	)

	if err := md.Convert(data, &buf); err != nil {
		return "", err
	}

	return buf.String(), nil
}
