package main

import (
	"bytes"
	"database/sql"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"

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
