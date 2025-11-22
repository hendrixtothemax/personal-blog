package main

import (
	"bytes"
	"database/sql"
	"fmt"
	"html/template"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	chromahtml "github.com/alecthomas/chroma/v2/formatters/html"
	"github.com/gorilla/mux"
	_ "github.com/mattn/go-sqlite3"
	"github.com/yuin/goldmark"
	highlighting "github.com/yuin/goldmark-highlighting/v2"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/renderer/html"
)

func main() {
	r := mux.NewRouter()

	db := startDB()

	// Use r.HandleFunc instead of http.Handle
	r.Handle("/foo", LoggingMiddleware(http.HandlerFunc(fooHandler)))
	// r.Handle("/", LoggingMiddleware(http.HandlerFunc(indexHandler)))
	r.Handle("/", ChainMiddleware(indexHandler(db), LoggingMiddleware))
	r.Handle("/blog", ChainMiddleware(blogHandler(db), LoggingMiddleware))
	r.Handle("/blog/{id}", ChainMiddleware(blogPostHandlerID(db), LoggingMiddleware))
	r.Handle("/login", LoggingMiddleware(http.HandlerFunc(loginHandler)))
	r.Handle("/logout", ChainMiddleware(logoutHandler(db), LoggingMiddleware))
	r.Handle("/favicon.ico", LoggingMiddleware(http.HandlerFunc(faviconHandler)))
	r.Handle("/js/htmx.js", LoggingMiddleware(http.HandlerFunc(htmxHandler)))
	r.Handle("/css/index.css", LoggingMiddleware(http.HandlerFunc(cssHandler)))
	r.Handle("/testmd", LoggingMiddleware(http.HandlerFunc(testMDHandler)))
	r.Handle("/user/register", LoggingMiddleware(http.HandlerFunc(registerUser(db))))
	r.Handle("/user/login", LoggingMiddleware(http.HandlerFunc(loginUser(db))))

	// Add the tag suggestion endpoint
	// Pass the db connection to tagSuggestHandler
	r.Handle("/tags/suggest", ChainMiddleware(tagSuggestHandler(db), LoggingMiddleware))
	// Update the createPost handler to use ChainMiddleware and pass db
	r.Handle("/create/post/push", ChainMiddleware(createPost(db), LoggingMiddleware))
	// Make sure createPostPageHandler is also chained if not already
	r.Handle("/create/post", ChainMiddleware(createPostPageHandler(db), LoggingMiddleware))

	// Use path variable!
	r.Handle("/htmx/{filename}", LoggingMiddleware(http.HandlerFunc(htmxTemplateHandler)))

	fmt.Println("Server Starting! localhost:8080/")
	log.Fatal(http.ListenAndServe("0.0.0.0:8080", r)) // Pass your mux.Router
}

func fooHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Cache-Control", "no-cache")
	w.Header().Add("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte("Foo"))
}

func indexHandler(db *sql.DB) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Set headers
		w.Header().Add("Cache-Control", "must-revalidate")
		w.Header().Add("Content-Type", "text/html; charset=utf-8")

		// Get user info if authenticated
		user, err0 := getUserFromSession(r, db) // Ignore error if anonymous

		if err0 != nil {
			fmt.Printf("Error Getting User: %s \n", err0)
		}

		data := TemplateData{
			IsAuthenticated: user != nil,
			User:            user,
		}

		// Parse all needed templates
		tmpl := template.Must(template.ParseFiles(
			"template/base.en.html",   // defines "base"
			"template/navbar.en.html", // partial navbar
			"template/index.en.html",  // overrides blocks in base
		))

		// Execute the base template which uses blocks from index.en.html
		err := tmpl.ExecuteTemplate(w, "base.en.html", data)
		if err != nil {
			http.Error(w, fmt.Sprintf("Template render error: %v", err), http.StatusInternalServerError)
			return
		}
	})
}

func blogHandler(db *sql.DB) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Set headers
		w.Header().Add("Cache-Control", "must-revalidate")
		w.Header().Add("Content-Type", "text/html; charset=utf-8")

		// Get user info if authenticated
		user, err0 := getUserFromSession(r, db) // Ignore error if anonymous

		if err0 != nil {
			fmt.Printf("Error Getting User: %s \n", err0)
		}

		posts, err := getPosts(db)
		if err != nil {
			http.Error(w, fmt.Sprintf("Error Getting Posts: %v", err), http.StatusInternalServerError)
			return
		}

		data := TemplateData{
			IsAuthenticated: user != nil,
			User:            user,
			Data: map[string]interface{}{
				"Posts": posts,
			},
		}

		// Parse all needed templates
		tmpl := template.Must(template.ParseFiles(
			"template/base.en.html",   // defines "base"
			"template/navbar.en.html", // partial navbar
			"template/blog.en.html",   // overrides blocks in base
			"template/postsummary.en.html",
		))

		// Execute the base template which uses blocks from index.en.html
		err = tmpl.ExecuteTemplate(w, "base.en.html", data)
		if err != nil {
			http.Error(w, fmt.Sprintf("Template render error: %v", err), http.StatusInternalServerError)
			return
		}
	})
}

func blogPostHandlerID(db *sql.DB) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Set headers
		w.Header().Add("Cache-Control", "must-revalidate")
		w.Header().Add("Content-Type", "text/html; charset=utf-8")

		post_id := mux.Vars(r)["id"]

		post_id_int, err := strconv.Atoi(post_id)
		if err != nil {
			http.Error(w, "Invalid blog ID", http.StatusBadRequest)
			return
		}

		// Get user info if authenticated
		user, err0 := getUserFromSession(r, db) // Ignore error if anonymous

		if err0 != nil {
			fmt.Printf("Error Getting User: %s \n", err0)
		}

		post, err := getPost(db, post_id_int)
		if err != nil {
			http.Error(w, fmt.Sprintf("Error Getting Post: %v", err), http.StatusInternalServerError)
			return
		}

		data := TemplateData{
			IsAuthenticated: user != nil,
			User:            user,
			Data: map[string]interface{}{
				"Post": post,
			},
		}

		// Parse all needed templates
		tmpl := template.Must(template.ParseFiles(
			"template/base.en.html",   // defines "base"
			"template/navbar.en.html", // partial navbar
			"template/post.en.html",
		))

		// Execute the base template which uses blocks from index.en.html
		err = tmpl.ExecuteTemplate(w, "base.en.html", data)
		if err != nil {
			http.Error(w, fmt.Sprintf("Template render error: %v", err), http.StatusInternalServerError)
			return
		}
	})
}

func saveUploadedFile(file multipart.File, header *multipart.FileHeader, uploadDir string) (string, string, error) {
	defer file.Close()

	// Reset file pointer to start (in case it's been partially read earlier)
	if seeker, ok := file.(io.Seeker); ok {
		_, err := seeker.Seek(0, io.SeekStart)
		if err != nil {
			return "", "", fmt.Errorf("failed to reset file pointer: %w", err)
		}
	}

	// Sanitize original filename
	ext := filepath.Ext(header.Filename)
	base := strings.TrimSuffix(filepath.Base(header.Filename), ext)

	// Add timestamp in milliseconds
	timestamp := time.Now().UnixNano() / 1e6
	filename := fmt.Sprintf("%s-%d%s", base, timestamp, ext)

	// Ensure upload directory exists
	err := os.MkdirAll(uploadDir, 0755)
	if err != nil {
		return "", "", fmt.Errorf("failed to create upload dir: %w", err)
	}

	// Create destination file
	path := filepath.Join(uploadDir, filename)
	dst, err := os.Create(path)
	if err != nil {
		return "", "", fmt.Errorf("failed to create destination file: %w", err)
	}
	defer dst.Close()

	// Copy the uploaded file contents to the destination file
	_, err = io.Copy(dst, file)
	if err != nil {
		return "", "", fmt.Errorf("failed to save file: %w", err)
	}

	return path, filename, nil
}

// createPost handles the submission of a new blog post, including file upload and tag management.
func createPost(db *sql.DB) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		const maxUploadSize = 50 << 20 // 50 MB
		r.Body = http.MaxBytesReader(w, r.Body, maxUploadSize)

		w.Header().Set("Cache-Control", "must-revalidate")
		w.Header().Set("Content-Type", "text/html; charset=utf-8")

		// Check auth
		user, err := getUserFromSession(r, db)
		if err != nil || user == nil {
			http.Error(w, "Forbidden: User not authenticated.", http.StatusForbidden)
			return
		}

		err = r.ParseMultipartForm(50 << 20) // 50 MB
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to parse form data: %v", err), http.StatusBadRequest)
			return
		}

		title := r.FormValue("post-title")
		if title == "" {
			http.Error(w, "Missing title", http.StatusBadRequest)
			return
		}

		summary := r.FormValue("post-summary")
		if summary == "" { // Changed from `title == ""` to `summary == ""`
			http.Error(w, "Missing summary", http.StatusBadRequest)
			return
		}

		public := r.FormValue("post-public")
		publicStatus := "0" // Default to private
		if public == "on" {
			publicStatus = "1"
		}

		// Get the tags input string
		tagsInput := r.FormValue("post-tags") // New: Get the tags from the form

		file, header, err := r.FormFile("postfile")
		if err != nil {
			http.Error(w, "Error retrieving markdown file", http.StatusBadRequest)
			return
		}
		defer file.Close()

		// Read file contents (though we are saving it, often good to have content temporarily)
		// No longer explicitly reading into `content` string, `saveUploadedFile` will handle the read from `file`
		// Your `saveUploadedFile` function assumes the file pointer is at the beginning.
		// If you also want `content` for processing, you'd need to copy `file` into a bytes.Buffer
		// before passing it to `saveUploadedFile`, or seek to start.
		// For now, let's assume `saveUploadedFile` is primary and we don't need `content` here.

		uploadDir := "./posts"
		savedPath, savedFileName, err := saveUploadedFile(file, header, uploadDir)
		if err != nil {
			log.Printf("Failed to save file: %v", err)
			http.Error(w, "Failed to save post file", http.StatusInternalServerError)
			return
		}
		log.Printf("File saved to: %s as %s", savedPath, savedFileName)

		// --- Start Database Transaction ---
		tx, err := db.Begin()
		if err != nil {
			log.Printf("Failed to start transaction: %v", err)
			http.Error(w, "Database error", http.StatusInternalServerError)
			return
		}
		defer tx.Rollback() // Ensure rollback on error

		// 1. Insert the Post
		// Note: `created_at` is handled by `DEFAULT CURRENT_TIMESTAMP` in your schema
		postStmt, err := tx.Prepare("INSERT INTO posts (title, summary, file_loc, public) VALUES (?, ?, ?, ?)")
		if err != nil {
			log.Printf("Failed to prepare post insert statement: %v", err)
			http.Error(w, "Database error", http.StatusInternalServerError)
			return
		}
		defer postStmt.Close()

		result, err := postStmt.Exec(title, summary, savedFileName, publicStatus)
		if err != nil {
			log.Printf("Failed to execute post insert statement: %v", err)
			http.Error(w, "Database error", http.StatusInternalServerError)
			return
		}

		postID, err := result.LastInsertId()
		if err != nil {
			log.Printf("Failed to get last insert ID for post: %v", err)
			http.Error(w, "Database error", http.StatusInternalServerError)
			return
		}

		// 2. Process and Associate Tags
		parsedTags := parseTags(tagsInput) // Helper function defined below
		log.Printf("Parsed tags: %v for post ID: %d", parsedTags, postID)

		insertPostTagStmt, err := tx.Prepare("INSERT INTO posts_tags (post_id, tag_id) VALUES (?, ?)")
		if err != nil {
			log.Printf("Failed to prepare posts_tags insert statement: %v", err)
			http.Error(w, "Database error", http.StatusInternalServerError)
			return
		}
		defer insertPostTagStmt.Close()

		for _, tagName := range parsedTags {
			if tagName == "" {
				continue // Skip empty tags after trimming/cleaning
			}

			// Check if tag exists
			var tagID int64
			err := tx.QueryRow("SELECT tag_id FROM tags WHERE tag_name = ?", tagName).Scan(&tagID)

			if err == sql.ErrNoRows {
				// Tag does not exist, create it
				insertTagStmt, err := tx.Prepare("INSERT INTO tags (tag_name) VALUES (?)")
				if err != nil {
					log.Printf("Failed to prepare tag insert statement: %v", err)
					http.Error(w, "Database error", http.StatusInternalServerError)
					return
				}
				tagResult, err := insertTagStmt.Exec(tagName)
				insertTagStmt.Close() // Close immediately
				if err != nil {
					log.Printf("Failed to execute tag insert statement for '%s': %v", tagName, err)
					http.Error(w, "Database error", http.StatusInternalServerError)
					return
				}
				tagID, err = tagResult.LastInsertId()
				if err != nil {
					log.Printf("Failed to get last insert ID for new tag '%s': %v", tagName, err)
					http.Error(w, "Database error", http.StatusInternalServerError)
					return
				}
				log.Printf("Created new tag: %s with ID %d", tagName, tagID)
			} else if err != nil {
				log.Printf("Query for tag '%s' failed: %v", tagName, err)
				http.Error(w, "Database error", http.StatusInternalServerError)
				return
			} else {
				log.Printf("Found existing tag: %s with ID %d", tagName, tagID)
			}

			// Insert into posts_tags join table
			_, err = insertPostTagStmt.Exec(postID, tagID)
			if err != nil {
				// Handle potential duplicate key error (if a tag was somehow added multiple times in input)
				// For SQLite, it might look like "UNIQUE constraint failed: posts_tags.post_id, posts_tags.tag_id"
				if strings.Contains(err.Error(), "UNIQUE constraint failed") {
					log.Printf("Warning: Duplicate posts_tags entry for post_id %d, tag_id %d. Skipping.", postID, tagID)
				} else {
					log.Printf("Failed to insert into posts_tags for post_id %d, tag_id %d: %v", postID, tagID, err)
					http.Error(w, "Database error", http.StatusInternalServerError)
					return
				}
			}
		}

		// Commit the transaction
		if err := tx.Commit(); err != nil {
			log.Printf("Failed to commit transaction: %v", err)
			http.Error(w, "Database error", http.StatusInternalServerError)
			return
		}

		http.Redirect(w, r, "/blog", http.StatusSeeOther) // Redirect to the blog page after successful creation
	})
}

// parseTags helper function to clean and split tags
func parseTags(tagsInput string) []string {
	tagsInput = strings.TrimSpace(tagsInput)
	if tagsInput == "" {
		return []string{}
	}

	rawTags := strings.Split(tagsInput, ",")
	var cleanedTags []string
	for _, tag := range rawTags {
		trimmedTag := strings.TrimSpace(tag)
		noSpaceTag := strings.ReplaceAll(trimmedTag, " ", "") // Remove all internal spaces
		if noSpaceTag != "" {
			cleanedTags = append(cleanedTags, noSpaceTag)
		}
	}
	return cleanedTags
}

func createPostPageHandler(db *sql.DB) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Set headers
		w.Header().Add("Cache-Control", "must-revalidate")
		w.Header().Add("Content-Type", "text/html; charset=utf-8")

		// Get user info if authenticated
		user, err0 := getUserFromSession(r, db) // Ignore error if anonymous

		if err0 != nil {
			fmt.Printf("Error Getting User: %s \n", err0)
		}

		data := TemplateData{
			IsAuthenticated: user != nil,
			User:            user,
			// Data: map[string]interface{}{
			// 	"Post": post,
			// },
		}

		if !data.IsAuthenticated {
			http.Redirect(w, r, "/login", http.StatusFound)
			return
		}

		// Parse all needed templates
		tmpl := template.Must(template.ParseFiles(
			"template/base.en.html",   // defines "base"
			"template/navbar.en.html", // partial navbar
			"template/postcreator.en.html",
		))

		// Execute the base template which uses blocks from index.en.html
		err := tmpl.ExecuteTemplate(w, "base.en.html", data)
		if err != nil {
			http.Error(w, fmt.Sprintf("Template render error: %v", err), http.StatusInternalServerError)
			return
		}
	})
}

func loginHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Cache-Control", "no-cache")
	w.Header().Add("Content-Type", "text/html; charset=utf-8")

	file, err := os.Open("./login.html")
	if err != nil {
		http.Error(w, "Something went wrong!", http.StatusInternalServerError)
		return
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		http.Error(w, "Something went wrong!", http.StatusInternalServerError)
		return
	}

	w.Write(data)
}

func logoutHandler(db *sql.DB) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, session_id, err := hasSession(r, db)

		if err != nil {
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}

		db.Exec("DELETE FROM sessions WHERE session_id = ?", session_id)

		http.SetCookie(w, &http.Cookie{
			Name:     "session_id",
			Value:    "",
			HttpOnly: true,
			Secure:   false, // only if using HTTPS
			Path:     "/",
			SameSite: http.SameSiteLaxMode,
			MaxAge:   -1,
		})

		http.Redirect(w, r, "/", http.StatusFound)
	})
}

func htmxHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "text/javascript; charset=utf-8")

	file, err := os.Open("./htmx.min.js")
	if err != nil {
		http.Error(w, "Something went wrong!", http.StatusInternalServerError)
		return
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		http.Error(w, "Something went wrong!", http.StatusInternalServerError)
		return
	}

	w.Write(data)
}

func cssHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "text/css; charset=utf-8")

	file, err := os.Open("./index.css")
	if err != nil {
		http.Error(w, "Something went wrong!", http.StatusInternalServerError)
		return
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		http.Error(w, "Something went wrong!", http.StatusInternalServerError)
		return
	}

	w.Write(data)
}

func faviconHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "image/png; charset=utf-8")

	file, err := os.Open("./favicon-blue-x64.png")
	if err != nil {
		http.Error(w, "Something went wrong!", http.StatusInternalServerError)
		return
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		http.Error(w, "Something went wrong!", http.StatusInternalServerError)
		return
	}

	w.Write(data)
}

func testMDHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	file, err := os.Open("./test.md")
	if err != nil {
		http.Error(w, "Something went wrong!", http.StatusInternalServerError)
		return
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		http.Error(w, "Something went wrong!", http.StatusInternalServerError)
		return
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
		http.Error(w, "Failed to render markdown", http.StatusInternalServerError)
		return
	}

	w.Write(buf.Bytes())
}

func htmxTemplateHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Cache-Control", "no-cache")
	w.Header().Add("Content-Type", "text/html; charset=utf-8")

	vars := mux.Vars(r)
	filename := vars["filename"]
	path := fmt.Sprintf("./htmx-template/%s.html", filename)

	file, err := os.Open(path)
	if err != nil {
		http.Error(w, "Something went wrong!", http.StatusInternalServerError)
		return
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		http.Error(w, "Something went wrong!", http.StatusInternalServerError)
		return
	}

	w.Write(data)
}

// tagSuggestHandler handles HTMX requests for tag suggestions.
func tagSuggestHandler(db *sql.DB) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		tagNameInput := r.FormValue("post-tags") // 'post-tags' is the name of the input field from HTML

		// If the user enters multiple tags separated by commas, we'll suggest based on the last one
		parts := strings.Split(tagNameInput, ",")
		lastTagFragment := strings.TrimSpace(parts[len(parts)-1])

		if lastTagFragment == "" {
			fmt.Fprint(w, "") // No suggestions if the last part is empty
			return
		}

		// Query for tags where tag_name starts with the last entered tag fragment
		query := "SELECT tag_name FROM tags WHERE tag_name LIKE ? ORDER BY tag_name LIMIT 5"
		rows, err := db.Query(query, lastTagFragment+"%")
		if err != nil {
			log.Printf("Query error in tagSuggestHandler: %v", err)
			http.Error(w, "Query error", http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		var suggestions []string
		for rows.Next() {
			var tagName string
			if err := rows.Scan(&tagName); err != nil {
				log.Printf("Error scanning tag in tagSuggestHandler: %v", err)
				continue // Skip problematic rows
			}
			suggestions = append(suggestions, tagName)
		}

		// Render suggestions as a simple list
		if len(suggestions) > 0 {
			// Start the suggestion list container
			fmt.Fprint(w, `<div class="tag-suggestions-list">`)
			for _, s := range suggestions {
				// Reconstruct the input value for client-side update
				// All tags except the last fragment
				currentTagsPrefix := strings.Join(parts[:len(parts)-1], ",")
				var newInputValue string
				if currentTagsPrefix == "" {
					newInputValue = s + ", " // Just the new tag if it's the first
				} else {
					// Append to existing tags with a comma and space
					newInputValue = currentTagsPrefix + ", " + s + ", "
				}

				// The hx-on:click event sets the input value and clears the suggestions
				fmt.Fprintf(w, `
                    <div
                        class="tag-suggestion-item"
                        hx-on:click="this.closest('#input-container').querySelector('#post-tags').value = '%s'; this.parentElement.innerHTML='';"
                    >%s</div>`, template.JSEscapeString(newInputValue), s) // Use JSEscapeString for safety
			}
			fmt.Fprint(w, `</div>`)
		} else {
			fmt.Fprint(w, `<div class="tag-suggestions-list">No matching tags.</div>`)
		}
	})
}
