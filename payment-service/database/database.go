package database

import (
	"database/sql"
	"log"
	"time"
)

func InitDb(url string) *sql.DB {
	db, err := sql.Open("postgres", url)
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}

	for i := 0; i < 10; i++ {
		if err := db.Ping(); err == nil {
			break
		}
		log.Printf("Waiting for DB... %d/10", i+1)
		time.Sleep(3 * time.Second)
	}

	migrations := []string{
		`CREATE TABLE IF NOT EXISTS payments (
			id UUID PRIMARY KEY,
			order_id UUID NOT NULL,
			user_id VARCHAR(255) NOT NULL,
			amount DECIMAL(10,2) NOT NULL,
			status VARCHAR(50) NOT NULL,
			created_at TIMESTAMP DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS user_balances (
			user_id VARCHAR(255) PRIMARY KEY,
			balance DECIMAL(10,2) NOT NULL DEFAULT 1000.00
		)`,
	}

	for _, m := range migrations {
		if _, err := db.Exec(m); err != nil {
			log.Fatalf("Migration failed: %v", err)
		}
	}

	log.Println("Payment DB initialized")
	return db
}
