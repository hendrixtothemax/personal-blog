package main

import (
	"fmt"
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
)

func main() {
	http.Handle("/foo", http.HandlerFunc(fooHandler))

	wrapped := http.HandlerFunc(indexHandler)
	http.Handle("/", LoggingMiddleware(wrapped))

	wrapped = http.HandlerFunc(loginHandler)
	http.Handle("/login", LoggingMiddleware(wrapped))

	wrapped = http.HandlerFunc(faviconHandler)
	http.Handle("/favicon.ico", LoggingMiddleware(wrapped))

	wrapped = http.HandlerFunc(htmxHandler)
	http.Handle("/js/htmx.js", LoggingMiddleware(wrapped))

	wrapped = http.HandlerFunc(cssHandler)
	http.Handle("/css/index.css", LoggingMiddleware(wrapped))

	wrapped = http.HandlerFunc(testMDHandler)
	http.Handle("/testmd", LoggingMiddleware(wrapped))

	startDB()

	fmt.Println("Server Starting! localhost:8080/")
	http.ListenAndServe("0.0.0.0:8080", nil)
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
		created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
		updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
		last_login TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
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


