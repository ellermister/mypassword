package main

import (
	"database/sql"
	"fmt"
	_ "github.com/mattn/go-sqlite3"
	webview "github.com/webview/webview_go"
	"log"
	"net/url"
	"os"
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

type FileDoc struct {
	Id      int    `json:"id"`
	Title   string `json:"title"`
	Content string `json:"content"`
}

func sqlConnectDB(path string, key string) (*sql.DB, error) {
	key = url.QueryEscape(key)
	dbname := fmt.Sprintf("%s?_cipher=sqlcipher&_legacy=3&_hmac_use=off&_kdf_iter=4000&_legacy_page_size=1024&_key=%s", path, key)
	db, err := sql.Open("sqlite3", dbname)
	if err != nil {
		log.Fatalf("Open Error %v\n", err)
		return nil, err
	}
	return db, nil
}

func sqlCreateTable(db *sql.DB) bool {
	sqlText := `
CREATE TABLE IF NOT EXISTS "main"."docs" (
"id"  INTEGER PRIMARY KEY autoincrement,
"title"  TEXT,
"content"  TEXT,
"status"  INTEGER,
"created_at"  TEXT,
"updated_at"  TEXT
)
;`

	_, err := db.Exec(sqlText)
	if err != nil {
		log.Fatal("Failed to create table:", err)
		return false
	}
	log.Printf("created docs successfully!")
	return true
}

func sqlCreateFile(db *sql.DB, title string, content string) bool {
	sqlText := `
		INSERT INTO docs ('title', 'content', 'status', 'created_at', 'updated_at')
		VALUES (?, ?, ?, ?, ?)
	`

	currentTime := time.Now()
	//  执行 SQL 语句，传递实际参数
	result, err := db.Exec(sqlText, title, content, 1, currentTime, currentTime)
	if err != nil {
		log.Fatalf("Error executing SQL: %v", err)
		return false
	}

	id, err := result.LastInsertId()
	if err != nil {
		log.Fatalf("Error retrieving last insert ID: %v", err)
		return false
	}

	log.Printf("Record inserted with ID: %d\n", id)

	return true
}

func sqlDeleteFile(db *sql.DB, id int) bool {
	sqlText := `DELETE FROM docs WHERE id=?`
	_, err := db.Exec(sqlText, id)
	if err != nil {
		log.Fatalf("Error executing SQL: %v", err)
		return false
	}
	return true
}

func sqlUpdateFile(db *sql.DB, id int, title string, content string) bool {
	sqlText := `UPDATE docs SET content=?, title=?,updated_at=? WHERE id=?`
	currentTime := time.Now()
	_, err := db.Exec(sqlText, content, title, currentTime, id)
	if err != nil {
		log.Fatalf("Error executing SQL: %v", err)
		return false
	}
	return true
}

func sqlGetOneFile(db *sql.DB, id int) (*FileDoc, error) {
	sqlText := "SELECT id,title,content FROM docs where id = ?"
	log.Printf("query docs for id : %d", id)

	var fileDoc FileDoc
	err := db.QueryRow(sqlText, id).Scan(&fileDoc.Id, &fileDoc.Title, &fileDoc.Content)

	if err != nil {
		log.Printf("Error executing SQL: %v", err)
		return nil, err
	}
	return &fileDoc, nil
}

func sqlGetAllFile(db *sql.DB, keyword string) []FileDoc {
	var sqlText string
	var args []interface{}

	if keyword == "" {
		sqlText = "SELECT id, title, content FROM docs"
	} else {
		sqlText = "SELECT id, title, content FROM docs WHERE title LIKE ? OR content LIKE ?"
		keyword = "%" + keyword + "%" // 为关键字添加通配符
		args = append(args, keyword, keyword)
	}

	// 执行查询
	result, err := db.Query(sqlText, args...)
	if err != nil {
		log.Fatalf("Error executing SQL: %v", err)
		fmt.Printf("ddd %v", []FileDoc{})
		return make([]FileDoc, 0)
	}
	defer result.Close()

	// 处理查询结果
	var rows = make([]FileDoc, 0)
	for result.Next() {
		var row FileDoc
		err = result.Scan(&row.Id, &row.Title, &row.Content)
		if err != nil {
			log.Fatal(err)
		}
		rows = append(rows, row)
	}
	return rows
}

func sqlIsCreatedDB(path string) bool {
	_, err := os.Stat(path)
	if err != nil {
		return false
	}
	return true
}

func sqlCreateDB(path string, password string) bool {
	if !sqlIsCreatedDB(path) {
		db, err := sqlConnectDB(path, password)
		if err != nil {
			log.Fatalf("Error connecting to database: %v", err)
			return false
		}
		DBConnect = db

		sqlCreateTable(db)
		return true
	}
	return false
}

func sqlTestDBConnect(db *sql.DB) bool {
	var i string
	err := db.QueryRow("SELECT SQLITE_VERSION()").Scan(&i)
	if err != nil {
		return false
		//log.Fatal("version query error:", err)
	}
	return true
	//fmt.Println("SQLITE_VERSION() =", i) // pseudorandom
}

func sqlLoginDB(path string, password string) bool {
	if sqlIsCreatedDB(path) {
		db, err := sqlConnectDB(path, password)
		if err != nil {
			log.Fatalf("Error connecting to database: %v", err)
			return false
		}
		DBConnect = db

		if sqlTestDBConnect(db) {
			return true
		}
		return false
	}
	return false
}

var (
	DBPath    = "./pass.db"
	DBConnect *sql.DB
	version   = "1.0"
	windowW   = 960
	windowH   = 500

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

func main() {
	fmt.Printf("sqlIsCreatedDB(DBPath) %v", sqlIsCreatedDB(DBPath))

	w := webview.New(false)
	defer w.Destroy()

	fmt.Printf("hwnd %d", w.Window())

	var hWnd = uintptr(w.Window()) // 替换为您的窗口句柄

	err := SetWindowDisplayAffinity(hWnd, WDA_EXCLUDEFROMCAPTURE)
	if err != nil {
		fmt.Println("Error:", err)
	} else {
		fmt.Println("Window is now excluded from capture.")
	}

	w.SetTitle("My password - " + version)
	w.SetSize(windowW, windowH, webview.HintNone)

	w.Bind("sqlGetAllFile", func(keyword string) []FileDoc {
		return sqlGetAllFile(DBConnect, keyword)
	})
	w.Bind("sqlGetOneFile", func(id int) FileDoc {
		doc, err := sqlGetOneFile(DBConnect, id)
		if err != nil {
			return FileDoc{}
		}
		return FileDoc{Id: id, Title: doc.Title, Content: doc.Content}
	})
	w.Bind("sqlUpdateFile", func(id int, title string, content string) bool {
		result := sqlUpdateFile(DBConnect, id, title, content)
		if result == false {
			return false
		}
		return true
	})
	w.Bind("sqlDeleteFile", func(id int) bool {
		result := sqlDeleteFile(DBConnect, id)
		if result == false {
			return false
		}
		return true
	})
	w.Bind("sqlCreateFile", func(title string, content string) bool {
		result := sqlCreateFile(DBConnect, title, content)
		if result == false {
			return false
		}
		return true
	})

	w.Bind("setTitle", func(title string) {
		w.SetTitle(title)
	})

	w.Bind("sqlIsCreatedDB", func() bool {
		return sqlIsCreatedDB(DBPath)
	})
	w.Bind("sqlCreateDB", func(password string) bool {
		return sqlCreateDB(DBPath, password)
	})
	w.Bind("sqlLoginDB", func(password string) bool {
		return sqlLoginDB(DBPath, password)
	})
	w.Bind("goMainPage", func() {
		w.SetHtml(uiHtml)
	})

	w.SetHtml(passwordHtml)

	w.Run()
}
