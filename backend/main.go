package main

import (
	"fmt"
	"github.com/gorilla/mux"
	"net/http"
	"os"
	"io"
	"bytes"
    "github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/renderer/html"
	highlighting "github.com/yuin/goldmark-highlighting/v2"
	chromahtml "github.com/alecthomas/chroma/v2/formatters/html"
	"database/sql"
	_ "github.com/mattn/go-sqlite3"
	"log"
	"html/template"
)

func main() {
	r := mux.NewRouter()

	// Use r.HandleFunc instead of http.Handle
	r.Handle("/foo", LoggingMiddleware(http.HandlerFunc(fooHandler)))
	r.Handle("/", LoggingMiddleware(http.HandlerFunc(indexHandler)))
	r.Handle("/login", LoggingMiddleware(http.HandlerFunc(loginHandler)))
	r.Handle("/favicon.ico", LoggingMiddleware(http.HandlerFunc(faviconHandler)))
	r.Handle("/js/htmx.js", LoggingMiddleware(http.HandlerFunc(htmxHandler)))
	r.Handle("/css/index.css", LoggingMiddleware(http.HandlerFunc(cssHandler)))
	r.Handle("/testmd", LoggingMiddleware(http.HandlerFunc(testMDHandler)))
	r.Handle("/user/create", LoggingMiddleware(http.HandlerFunc(createUser)))

	// Use path variable!
	r.Handle("/htmx/{filename}", LoggingMiddleware(http.HandlerFunc(htmxTemplateHandler)))

	startDB()

	fmt.Println("Server Starting! localhost:8080/")
	log.Fatal(http.ListenAndServe("0.0.0.0:8080", r))  // Pass your mux.Router
}

func startDB(){
	db, err := sql.Open("sqlite3", "./main.db")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	createTable := `
	CREATE TABLE IF NOT EXISTS users (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        email TEXT NOT NULL UNIQUE,
        password TEXT NOT NULL,
		salt TEXT NOT NULL,
		created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
		updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
		last_login TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
    );
	`

	createSessionTable := `
	PRAGMA foreign_keys = ON;

	CREATE TABLE IF NOT EXISTS sessions (
        session_id TEXT NOT NULL UNIQUE,
		user_id INTEGER NOT NULL,
		created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
		last_use TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
		FOREIGN KEY (user_id) REFERENCES users(user_id) ON DELETE CASCADE
    );
	`

	createTrigger := `
	CREATE TRIGGER IF NOT EXISTS trigger_update_users_updated_at
	AFTER UPDATE ON users
	FOR EACH ROW
	WHEN OLD.email != NEW.email OR OLD.password != NEW.password
	BEGIN
		UPDATE users
		SET updated_at = strftime('%Y-%m-%dT%H:%M:%SZ', 'now')
		WHERE id = OLD.id;
	END;
	`

	_, err = db.Exec(createTable)
	if err != nil {
		log.Fatal(err)
	}

	_, err = db.Exec(createSessionTable)
	if err != nil {
		log.Fatal(err)
	}

	_, err = db.Exec(createTrigger)
	if err != nil {
		log.Fatal(err)
	}

}

func fooHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Cache-Control", "no-cache")
	w.Header().Add("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte("Foo"))
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Cache-Control", "no-cache")
	w.Header().Add("Content-Type", "text/html; charset=utf-8")

	file, err := os.Open("./index.html")
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

func createUser(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Cache-Control", "no-cache")
	w.Header().Add("Content-Type", "text/html; charset=utf-8")

	if r.Method != http.MethodPost {
		http.Error(w, "Mehtod Not Allowed", http.StatusMethodNotAllowed)
		return
	}


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