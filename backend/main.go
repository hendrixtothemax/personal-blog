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
	r.Handle("/create/post", ChainMiddleware(createPostPageHandler(db), LoggingMiddleware))
	r.Handle("/create/post/push", ChainMiddleware(createPost(db), LoggingMiddleware))
	r.Handle("/login", LoggingMiddleware(http.HandlerFunc(loginHandler)))
	r.Handle("/logout", ChainMiddleware(logoutHandler(db), LoggingMiddleware))
	r.Handle("/favicon.ico", LoggingMiddleware(http.HandlerFunc(faviconHandler)))
	r.Handle("/js/htmx.js", LoggingMiddleware(http.HandlerFunc(htmxHandler)))
	r.Handle("/css/index.css", LoggingMiddleware(http.HandlerFunc(cssHandler)))
	r.Handle("/testmd", LoggingMiddleware(http.HandlerFunc(testMDHandler)))
	r.Handle("/user/register", LoggingMiddleware(http.HandlerFunc(registerUser(db))))
	r.Handle("/user/login", LoggingMiddleware(http.HandlerFunc(loginUser(db))))

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

func createPost(db *sql.DB) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		const maxUploadSize = 50 << 20 // 50 MB
		r.Body = http.MaxBytesReader(w, r.Body, maxUploadSize)

		// Set headers
		w.Header().Set("Cache-Control", "must-revalidate")
		w.Header().Set("Content-Type", "text/html; charset=utf-8")

		// Check auth
		user, err := getUserFromSession(r, db)
		if err != nil || user == nil {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}

		// Parse multipart form
		err = r.ParseMultipartForm(50 << 20) // 50 MB
		if err != nil {
			http.Error(w, "Failed to parse form data", http.StatusBadRequest)
			return
		}

		// Get the post title
		title := r.FormValue("post-title")
		if title == "" {
			http.Error(w, "Missing title", http.StatusBadRequest)
			return
		}

		// Get the post summary
		summary := r.FormValue("post-summary")
		if title == "" {
			http.Error(w, "Missing summary", http.StatusBadRequest)
			return
		}

		// Get the post public
		public := r.FormValue("post-public")
		if title == "" {
			http.Error(w, "Missing public", http.StatusBadRequest)
			return
		}

		// Get the uploaded file
		file, header, err := r.FormFile("postfile")
		if err != nil {
			http.Error(w, "Error retrieving file", http.StatusBadRequest)
			return
		}
		defer file.Close()

		// Read file contents
		var buf strings.Builder
		_, err = io.Copy(&buf, file)
		if err != nil {
			http.Error(w, "Error reading file", http.StatusInternalServerError)
			return
		}

		content := buf.String()

		fmt.Printf("Received post: title=%s, filename=%s, content length=%d\n",
			title, header.Filename, len(content))

		// Define where to save uploaded files
		uploadDir := "./posts"

		// Save file with timestamp
		savedPath, savedFileName, err := saveUploadedFile(file, header, uploadDir)
		if err != nil {
			http.Error(w, "Failed to save file", http.StatusInternalServerError)
			return
		}

		fmt.Printf("File saved to: %s as %s\n", savedPath, savedFileName)
		fmt.Println(public)

		public_status := "0"

		if public == "on" {
			public_status = "1"
		}

		_, err = db.Exec("INSERT INTO posts (title, summary, file_loc, public) VALUES (?, ?, ?, ?)", title, summary, savedFileName, public_status)
		if err != nil {
			http.Error(w, "Database error", http.StatusInternalServerError)
			return
		}

		// Success response
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("File uploaded successfully."))
	})
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
