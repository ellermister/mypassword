package main

import (
	"database/sql"
	"fmt"
	_ "github.com/mattn/go-sqlite3"
	webview "github.com/webview/webview_go"
	"log"
	"net/url"
	"os"
	"os/exec"
	"syscall"
	"time"
)
import (
	_ "embed"
)

//go:embed ui.html
var uiHtml string

//go:embed password.html
var passwordHtml string

type Document struct {
	Id      int    `json:"id"`
	Title   string `json:"title"`
	Content string `json:"content"`
}

var (
	DBPath       = "./pass.db"
	DBConnection *sql.DB
	version      = "1.1"
	windowTitle  = "My password v" + version
	windowWidth  = 960
	windowHeight = 500

	// 加载 user32.dll
	user32                       = syscall.NewLazyDLL("user32.dll")
	procSetWindowDisplayAffinity = user32.NewProc("SetWindowDisplayAffinity")
)

const (
	// 定义常量值
	WDA_EXCLUDEFROMCAPTURE = 0x00000011
)

// SetWindowDisplayAffinity 调用 Win32 函数
func SetWindowDisplayAffinity(hWnd uintptr, dwAffinity uint32) error {
	ret, _, err := procSetWindowDisplayAffinity.Call(hWnd, uintptr(dwAffinity))
	if ret == 0 {
		return fmt.Errorf("failed to set window display affinity: %v", err)
	}
	return nil
}

func connectToDatabase(path string, key string) (*sql.DB, error) {
	key = url.QueryEscape(key)
	dbURL := fmt.Sprintf("%s?_cipher=sqlcipher&_legacy=3&_hmac_use=off&_kdf_iter=4000&_legacy_page_size=1024&_key=%s", path, key)
	db, err := sql.Open("sqlite3", dbURL)
	if err != nil {
		log.Printf("Failed to connect to database: %v", err)
		return nil, err
	}
	return db, nil
}

func createDocsTable(db *sql.DB) error {
	query := `
CREATE TABLE IF NOT EXISTS "main"."docs" (
	"id" INTEGER PRIMARY KEY AUTOINCREMENT,
	"title" TEXT,
	"content" TEXT,
	"status" INTEGER,
	"created_at" TEXT,
	"updated_at" TEXT
);`
	_, err := db.Exec(query)
	if err != nil {
		return fmt.Errorf("failed to create table: %w", err)
	}
	log.Println("Docs table created successfully!")
	return nil
}

func addDocument(db *sql.DB, title, content string) (int64, error) {
	query := `
INSERT INTO docs (title, content, status, created_at, updated_at)
VALUES (?, ?, ?, ?, ?);`
	now := time.Now()
	result, err := db.Exec(query, title, content, 1, now, now)
	if err != nil {
		return 0, fmt.Errorf("failed to insert document: %w", err)
	}
	return result.LastInsertId()
}

func deleteDocument(db *sql.DB, id int) error {
	query := `DELETE FROM docs WHERE id=?`
	_, err := db.Exec(query, id)
	if err != nil {
		return fmt.Errorf("failed to delete document: %w", err)
	}
	return nil
}

func updateDocument(db *sql.DB, id int, title, content string) error {
	query := `UPDATE docs SET title=?, content=?, updated_at=? WHERE id=?`
	now := time.Now()
	_, err := db.Exec(query, title, content, now, id)
	if err != nil {
		return fmt.Errorf("failed to update document: %w", err)
	}
	return nil
}

func getDocument(db *sql.DB, id int) (*Document, error) {
	query := `SELECT id, title, content FROM docs WHERE id = ?`
	var doc Document
	err := db.QueryRow(query, id).Scan(&doc.Id, &doc.Title, &doc.Content)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch document: %w", err)
	}
	return &doc, nil
}

func searchDocuments(db *sql.DB, keyword string) ([]Document, error) {
	var query string
	var args []interface{}

	if keyword == "" {
		query = "SELECT id, title, content FROM docs"
	} else {
		query = "SELECT id, title, content FROM docs WHERE title LIKE ? OR content LIKE ?"
		keyword = "%" + keyword + "%"
		args = append(args, keyword, keyword)
	}

	rows, err := db.Query(query, args...)
	if err != nil {
		return make([]Document, 0), fmt.Errorf("failed to search documents: %w", err)
	}
	defer rows.Close()

	var results = make([]Document, 0)
	for rows.Next() {
		var doc Document
		if err := rows.Scan(&doc.Id, &doc.Title, &doc.Content); err != nil {
			return nil, fmt.Errorf("failed to scan document row: %w", err)
		}
		results = append(results, doc)
	}
	return results, nil
}

func isDatabaseInitialized(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func initializeDatabase(path, password string) (*sql.DB, error) {
	if isDatabaseInitialized(path) {
		return nil, fmt.Errorf("database already exists")
	}
	db, err := connectToDatabase(path, password)
	if err != nil {
		return nil, err
	}
	if err := createDocsTable(db); err != nil {
		return nil, err
	}
	return db, nil
}

func testDatabaseConnection(db *sql.DB) error {
	var version string
	if err := db.QueryRow("SELECT SQLITE_VERSION()").Scan(&version); err != nil {
		return fmt.Errorf("database connection test failed: %w", err)
	}
	return nil
}

func authenticateDatabase(path, password string) (*sql.DB, error) {
	db, err := connectToDatabase(path, password)
	if err != nil {
		return nil, err
	}
	if err := testDatabaseConnection(db); err != nil {
		return nil, err
	}
	return db, nil
}

func openURLByBrowser(url string) {
	cmd := exec.Command("cmd", "/c", "start", url)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	cmd.Start()
}

func main() {
	fmt.Printf("isDatabaseInitialized(DBPath) %v", isDatabaseInitialized(DBPath))

	w := webview.New(false)
	defer w.Destroy()

	var hWnd = uintptr(w.Window())

	if err := SetWindowDisplayAffinity(hWnd, WDA_EXCLUDEFROMCAPTURE); err != nil {
		fmt.Printf("Error setting window affinity: %e", err)
	}

	w.SetTitle(windowTitle)
	w.SetSize(windowWidth, windowHeight, webview.HintNone)

	w.Bind("ui_searchDocuments", func(keyword string) []Document {
		docs, _ := searchDocuments(DBConnection, keyword)
		return docs
	})
	w.Bind("ui_getDocument", func(id int) Document {
		doc, err := getDocument(DBConnection, id)
		if err != nil {
			return Document{}
		}
		return Document{Id: id, Title: doc.Title, Content: doc.Content}
	})
	w.Bind("ui_updateDocument", func(id int, title string, content string) bool {
		result := updateDocument(DBConnection, id, title, content)
		return result == nil
	})
	w.Bind("ui_deleteDocument", func(id int) bool {
		result := deleteDocument(DBConnection, id)
		return result == nil
	})
	w.Bind("ui_addDocument", func(title string, content string) bool {
		insertId, _ := addDocument(DBConnection, title, content)
		return insertId > 0
	})

	w.Bind("ui_setTitle", func(title string) {
		w.SetTitle(windowTitle + " - " + title)
	})

	w.Bind("ui_isDatabaseInitialized", func() bool {
		return isDatabaseInitialized(DBPath)
	})
	w.Bind("ui_initializeDatabase", func(password string) bool {
		db, err := initializeDatabase(DBPath, password)
		if err != nil {
			log.Printf("Failed to initialize database: %v", err)
			return false
		}
		DBConnection = db
		return true
	})
	w.Bind("ui_authenticateDatabase", func(password string) bool {
		db, err := authenticateDatabase(DBPath, password)
		if err != nil {
			log.Printf("Authentication failed: %v", err)
			return false
		}
		DBConnection = db
		return true
	})
	w.Bind("ui_goMainPage", func() {
		w.SetHtml(uiHtml)
	})

	w.Bind("ui_OpenSourceCodeURL", func() {
		openURLByBrowser("https://github.com/ellermister/mypassword")
	})

	w.SetHtml(passwordHtml)

	w.Run()
}
