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
		`CREATE TABLE IF NOT EXISTS deliveries (
			id UUID PRIMARY KEY,
			order_id UUID NOT NULL,
			user_id VARCHAR(255) NOT NULL,
			delivery_address TEXT,
			status VARCHAR(50) NOT NULL DEFAULT 'PENDING',
			tracking_number VARCHAR(50),
			current_location TEXT DEFAULT 'Processing',
			estimated_delivery VARCHAR(20),
			created_at TIMESTAMP DEFAULT NOW(),
			updated_at TIMESTAMP DEFAULT NOW()
		)`,
	}

	for _, m := range migrations {
		if _, err := db.Exec(m); err != nil {
			log.Fatalf("Migration failed: %v", err)
		}
	}

	log.Println("Delivery DB initialized")
	return db
}
