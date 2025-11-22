package main

import (
	"bytes"
	"database/sql"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"os"
	"strings"

	chromahtml "github.com/alecthomas/chroma/v2/formatters/html"
	"github.com/yuin/goldmark"
	highlighting "github.com/yuin/goldmark-highlighting/v2"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/renderer/html"

	"log"
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
	ID      int
	Title   string
	Date    string
	Summary string
	Content template.HTML
	FileLoc string
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
		var post_id int
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

		fmt.Printf("PID: %d | Title: %s | Created At: %s | File Loc: %s | Tag Numbs: %d\n", post_id, title, created_at, file_loc, len(tagList))

		curPost := Post{
			ID:      post_id,
			Title:   title,
			Summary: summary,
			Date:    created_at,
			FileLoc: file_loc,
			Topics:  tagList,
			Content: "",
		}

		posts = append(posts, curPost)

		numbRows += 1
	}

	fmt.Printf("Number of Rows: %d\n", numbRows)

	return &posts, nil
}

func getPost(db *sql.DB, postID int) (Post, error) {
	var post_id int
	var title string
	var summary string
	var created_at string
	var file_loc string
	var tags sql.NullString

	err := db.QueryRow(
		`
		SELECT p.post_id, p.title, p.summary, p.created_at, p.file_loc, 
			COALESCE(GROUP_CONCAT(t.tag_name), '') AS tags
		FROM (
			SELECT post_id, title, summary, created_at, file_loc
			FROM posts
			WHERE post_id = ?
		) AS p
		LEFT JOIN posts_tags pt ON pt.post_id = p.post_id
		LEFT JOIN tags t ON t.tag_id = pt.tag_id
		GROUP BY p.post_id, p.title, p.summary, p.created_at, p.file_loc
		`, postID,
	).Scan(&post_id, &title, &summary, &created_at, &file_loc, &tags)

	if err != nil {
		return Post{}, fmt.Errorf("db row error: %s", err)
	}

	tagList := []string{}
	if tags.Valid && tags.String != "" {
		tagList = strings.Split(tags.String, ",")
	}

	if len(tagList) < 1 {
		tagList = append(tagList, "None")
	}

	content, err := mdToHTML(file_loc)

	if err != nil {
		return Post{}, fmt.Errorf("md error: %s", err)
	}

	post := Post{
		ID:      post_id,
		Title:   title,
		Summary: summary,
		Date:    created_at,
		FileLoc: file_loc,
		Topics:  tagList,
		Content: content,
	}

	return post, nil

}

func mdToHTML(path string) (template.HTML, error) {
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

	return template.HTML(buf.String()), nil
}

// APIGetPosts handles fetching and filtering/sorting posts for HTMX requests.
// It's exported (uppercase A) so it can be called from main.
func APIGetPosts(db *sql.DB) http.Handler { // <--- NEW: Exported function
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Set headers to ensure fresh content and correct type for HTMX
		w.Header().Set("Cache-Control", "no-store, must-revalidate")
		w.Header().Set("Content-Type", "text/html; charset=utf-8")

		// Extract query parameters from the URL
		query := r.URL.Query().Get("q")
		searchBy := r.URL.Query().Get("search_by")
		sortBy := r.URL.Query().Get("sort_by")

		// Apply default values if parameters are not provided
		if searchBy == "" {
			searchBy = "name" // Default search by post name
		}
		if sortBy == "" {
			sortBy = "date-desc" // Default sort by most recent
		}

		log.Printf("API Get Posts request: q='%s', searchBy='%s', sortBy='%s'", query, searchBy, sortBy)

		// Fetch posts from the database based on the filters and sort order
		posts, err := getFilteredAndSortedPosts(db, query, searchBy, sortBy) // Call helper
		if err != nil {
			log.Printf("Error getting filtered and sorted posts: %v", err)
			http.Error(w, "Error fetching posts", http.StatusInternalServerError)
			return
		}

		// Prepare data for the template fragment
		data := TemplateData{
			Data: map[string]interface{}{
				"Posts": posts,
			},
		}

		// Parse ONLY the fragment templates that will be rendered by HTMX
		// postsummary.en.html is included because post_list_fragment.en.html uses it.
		// NOTE: Paths are relative to where the server is run, typically project root.
		tmpl := template.Must(template.ParseFiles("template/postsummary.en.html", "template/post_list_fragment.en.html"))

		// Execute the fragment template and write the HTML response
		err = tmpl.ExecuteTemplate(w, "post_list_fragment.en.html", data)
		if err != nil {
			log.Printf("Template render error for post_list_fragment.en.html: %v", err)
			http.Error(w, fmt.Sprintf("Template render error: %v", err), http.StatusInternalServerError)
			return
		}
	})
}

// getFilteredAndSortedPosts is a helper function that constructs and executes the database query.
// It's not exported (lowercase g) as it's only used internally by APIGetPosts.
func getFilteredAndSortedPosts(db *sql.DB, query, searchBy, sortBy string) (*[]Post, error) {
	var posts []Post
	var args []interface{}

	sqlQuery := `
		SELECT p.post_id, p.title, p.summary, p.created_at, p.file_loc, COALESCE(GROUP_CONCAT(t.tag_name), '') AS tags
		FROM posts AS p
		LEFT JOIN posts_tags pt ON pt.post_id = p.post_id
		LEFT JOIN tags t ON t.tag_id = pt.tag_id
		WHERE p.public = 1 `

	if query != "" {
		searchPattern := "%" + query + "%"
		if searchBy == "name" {
			sqlQuery += ` AND p.title LIKE ? `
			args = append(args, searchPattern)
		} else if searchBy == "tag" {
			sqlQuery += ` AND p.post_id IN (
				SELECT pt_inner.post_id
				FROM posts_tags pt_inner
				JOIN tags t_inner ON t_inner.tag_id = pt_inner.tag_id
				WHERE t_inner.tag_name LIKE ?
			) `
			args = append(args, searchPattern)
		}
	}

	sqlQuery += ` GROUP BY p.post_id, p.title, p.summary, p.created_at, p.file_loc `

	switch sortBy {
	case "date-asc":
		sqlQuery += ` ORDER BY p.created_at ASC `
	case "date-desc":
		sqlQuery += ` ORDER BY p.created_at DESC `
	case "alphabetical":
		sqlQuery += ` ORDER BY p.title ASC `
	default:
		sqlQuery += ` ORDER BY p.created_at DESC `
	}

	log.Printf("Executing SQL: %s with args: %v", sqlQuery, args)

	rows, err := db.Query(sqlQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("db query error in getFilteredAndSortedPosts: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var post_id int
		var title string
		var summary string
		var created_at string
		var file_loc string
		var tags sql.NullString

		err := rows.Scan(&post_id, &title, &summary, &created_at, &file_loc, &tags)
		if err != nil {
			return nil, fmt.Errorf("db row scan error in getFilteredAndSortedPosts: %w", err)
		}

		tagList := []string{}
		if tags.Valid && tags.String != "" {
			tagList = strings.Split(tags.String, ",")
		} else {
			// If no tags, ensure "None" is added if that's your convention for display.
			// Your postsummary template `{{if ne . "None"}}` correctly handles this.
			tagList = append(tagList, "None")
		}

		curPost := Post{
			ID:      post_id,
			Title:   title,
			Summary: summary,
			Date:    created_at,
			FileLoc: file_loc,
			Topics:  tagList,
		}
		posts = append(posts, curPost)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error in getFilteredAndSortedPosts: %w", err)
	}

	return &posts, nil
}
