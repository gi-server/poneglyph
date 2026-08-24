package database

import (
	"database/sql"
	"fmt"
	"log"
	"os"

	_ "github.com/lib/pq"
)

var DB *sql.DB

func ConnectDB() error {
	host := os.Getenv("DB_HOST")
	port := os.Getenv("DB_PORT")
	user := os.Getenv("DB_USER")
	password := os.Getenv("DB_PASSWORD")
	dbname := os.Getenv("DB_NAME")

	if host == "" {
		host = "localhost"
	}
	if port == "" {
		port = "5432"
	}
	if user == "" {
		user = "postgres"
	}
	if dbname == "" {
		dbname = "postgres"
	}

	psqlInfo := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		host, port, user, password, dbname,
		getEnvDB("DB_SSLMODE", "disable"), // set to 'require' or 'verify-full' in production
	)

	var err error
	DB, err = sql.Open("postgres", psqlInfo)
	if err != nil {
		return err
	}

	// Connection pool configuration — prevents too many open connections
	DB.SetMaxOpenConns(25)
	DB.SetMaxIdleConns(5)
	DB.SetConnMaxLifetime(5 * 60 * 1e9) // 5 minutes

	err = DB.Ping()
	if err != nil {
		return err
	}

	log.Println("Successfully connected to the database")
	return nil
}

func getEnvDB(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func InitSchema() error {
	schema := `
	CREATE TABLE IF NOT EXISTS users (
		id SERIAL PRIMARY KEY,
		username VARCHAR(50) UNIQUE NOT NULL,
		password_hash VARCHAR(255) NOT NULL,
		role VARCHAR(50) DEFAULT 'user',
		is_disabled BOOLEAN DEFAULT FALSE,
		admin_id INT REFERENCES users(id),
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS customers (
		id VARCHAR(50) PRIMARY KEY,
		workspace_id INT REFERENCES users(id),
		name VARCHAR(255) NOT NULL,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS documents (
		id SERIAL PRIMARY KEY,
		workspace_id INT REFERENCES users(id),
		filename VARCHAR(255) NOT NULL,
		filepath VARCHAR(512) NOT NULL,
		original_name VARCHAR(255) NOT NULL,
		status VARCHAR(50) DEFAULT 'uploaded',
		ocr_text TEXT,
		document_type VARCHAR(100),
		person_name VARCHAR(255),
		dob VARCHAR(50),
		document_id_number VARCHAR(100),
		confidence FLOAT,
		customer_id VARCHAR(50) REFERENCES customers(id),
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS audit_logs (
		id SERIAL PRIMARY KEY,
		workspace_id INT REFERENCES users(id),
		actor_id INT REFERENCES users(id),
		document_id INT REFERENCES documents(id),
		action VARCHAR(255) NOT NULL,
		details JSONB,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS document_shares (
		token VARCHAR(64) PRIMARY KEY,
		document_id INT REFERENCES documents(id),
		expires_at TIMESTAMP,
		single_use BOOLEAN DEFAULT FALSE,
		is_revoked BOOLEAN DEFAULT FALSE,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);
	`
	_, err := DB.Exec(schema)
	if err != nil {
		return fmt.Errorf("error creating schema: %w", err)
	}

	// 1. Add missing columns safely
	DB.Exec("ALTER TABLE users ADD COLUMN role VARCHAR(50) DEFAULT 'user'")
	DB.Exec("ALTER TABLE users ADD COLUMN is_disabled BOOLEAN DEFAULT FALSE")
	DB.Exec("ALTER TABLE users ADD COLUMN admin_id INT REFERENCES users(id)")
	
	DB.Exec("ALTER TABLE documents ADD COLUMN dob VARCHAR(50)")
	DB.Exec("ALTER TABLE documents ADD COLUMN document_id_number VARCHAR(100)")
	DB.Exec("ALTER TABLE documents ADD COLUMN customer_id VARCHAR(50) REFERENCES customers(id)")
	DB.Exec("ALTER TABLE audit_logs ADD COLUMN actor_id INT REFERENCES users(id)")

	// 2. Rename existing columns to workspace_id where appropriate
	// (Ignore errors if the column is already renamed or doesn't exist)
	DB.Exec("ALTER TABLE customers RENAME COLUMN user_id TO workspace_id")
	DB.Exec("ALTER TABLE documents RENAME COLUMN user_id TO workspace_id")
	DB.Exec("ALTER TABLE audit_logs RENAME COLUMN user_id TO workspace_id")

	log.Println("Database schema initialized")
	return nil
}
