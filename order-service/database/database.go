package database

import (
	"database/sql"
	"log"
)

func InitDb(dsn string) *sql.DB {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	if err := db.Ping(); err != nil {
		log.Fatalf("Failed to ping database: %v", err)
	}

	migrations := []string{
		`CREATE TABLE IF NOT EXISTS orders (
			id UUID PRIMARY KEY,
			user_id VARCHAR(255) NOT NULL,
			status VARCHAR(50) NOT NULL DEFAULT 'PENDING',
			total_amount DECIMAL(10,2) NOT NULL,
			delivery_address TEXT,
			created_at TIMESTAMP DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS delivery_statuses (
			order_id UUID PRIMARY KEY REFERENCES orders(id),
			status VARCHAR(50),
			tracking_number VARCHAR(255),
			estimated_delivery VARCHAR(100),
			updated_at TIMESTAMP DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS outbox_events (
			id UUID PRIMARY KEY,
			topic VARCHAR(255) NOT NULL,
			payload TEXT NOT NULL,
			sent BOOLEAN DEFAULT FALSE,
			created_at TIMESTAMP DEFAULT NOW()
		)`,
	}

	for _, migration := range migrations {
		if _, err := db.Exec(migration); err != nil {
			log.Fatalf("Failed to create migration table: %v", err)
		}
	}

	log.Printf("Database initialized successfully")
	return db
}
