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
	"golang.org/x/crypto/bcrypt"
	"crypto/rand"
	"encoding/base64"
	"strings"
)

func main() {
	r := mux.NewRouter()

	db := startDB()

	// Use r.HandleFunc instead of http.Handle
	r.Handle("/foo", LoggingMiddleware(http.HandlerFunc(fooHandler)))
	r.Handle("/", LoggingMiddleware(http.HandlerFunc(indexHandler)))
	r.Handle("/login", LoggingMiddleware(http.HandlerFunc(loginHandler)))
	r.Handle("/favicon.ico", LoggingMiddleware(http.HandlerFunc(faviconHandler)))
	r.Handle("/js/htmx.js", LoggingMiddleware(http.HandlerFunc(htmxHandler)))
	r.Handle("/css/index.css", LoggingMiddleware(http.HandlerFunc(cssHandler)))
	r.Handle("/testmd", LoggingMiddleware(http.HandlerFunc(testMDHandler)))
	r.Handle("/user/register", LoggingMiddleware(http.HandlerFunc(registerUser(db))))
	r.Handle("/user/login", LoggingMiddleware(http.HandlerFunc(loginUser(db))))

	// Use path variable!
	r.Handle("/htmx/{filename}", LoggingMiddleware(http.HandlerFunc(htmxTemplateHandler)))

	fmt.Println("Server Starting! localhost:8080/")
	log.Fatal(http.ListenAndServe("0.0.0.0:8080", r))  // Pass your mux.Router
}

func startDB() *sql.DB{
	db, err := sql.Open("sqlite3", "./main.db")
	if err != nil {
		log.Fatal(err)
	}

	createTable := `
	CREATE TABLE IF NOT EXISTS users (
        user_id INTEGER PRIMARY KEY AUTOINCREMENT,
        email TEXT NOT NULL UNIQUE,
        password TEXT NOT NULL,
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

	return db
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

		if err != nil{
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

		session_id, err := generateSession(db, user_id); 
		
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


        w.Write([]byte("Success"))
    }
}

func generateSession(db *sql.DB, userid int64) (string, error) {
	session_id, err := generateSessionID()
	if err != nil {
		return "", err
	}

	_, err = db.Exec("INSERT INTO sessions (session_id, user_id) VALUES (?, ?)", session_id, userid)
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
		var user_id int64

		err = db.QueryRow("SELECT password, user_id FROM users WHERE email = ? LIMIT 1", email).Scan(&hashedPassword, &user_id)

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
		session_id, err := generateSession(db, user_id)

		if err != nil {
			http.Error(w, "Failed to create session", http.StatusInternalServerError)
			return
		}

		res, err2 := db.Exec("UPDATE users SET (last_login = strftime('%Y-%m-%dT%H:%M:%SZ', 'now')) WHERE user_id = ?", user_id)
		if err2 != nil {
			log.Printf("Failed to update last_login: %v", err2)
		} else {
			rowsAffected, err2 := res.RowsAffected()
			if err2 != nil {
				log.Printf("Error checking rows affected: %v", err2)
			} else {
				log.Printf("last_login update successful; rows affected: %d", rowsAffected)
			}
		}

		http.SetCookie(w, &http.Cookie{
			Name:     "session_id",
			Value:    session_id,
			HttpOnly: true,
			Secure:   false, // only if using HTTPS
			Path:     "/",
			SameSite: http.SameSiteStrictMode,
		})

        w.Write([]byte("Success"))
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