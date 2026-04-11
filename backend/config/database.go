package config

import (
	"database/sql"
	"log"

	_ "github.com/go-sql-driver/mysql"
)

func InitDB() *sql.DB {
	dsn := "root:@tcp(localhost:3306)/doorlock_db"
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}

	err = db.Ping()
	if err != nil {
		log.Fatal("Failed to ping database:", err)
	}

	log.Println("Database connected successfully")
	return db
}
