package main

import (
	"database/sql"

	_ "github.com/mattn/go-sqlite3"
)

func main() {
    db, err := sql.Open("sqlite3", "test.db")
    if err != nil {
        panic(err)
    }
    defer db.Close()

    // Create tables
    _, err = db.Exec(`
        CREATE TABLE IF NOT EXISTS users (
            id INTEGER PRIMARY KEY,
            name TEXT NOT NULL,
            email TEXT UNIQUE
        );

        CREATE TABLE IF NOT EXISTS orders (
            id INTEGER PRIMARY KEY,
            user_id INTEGER,
            product TEXT NOT NULL,
            price DECIMAL(10,2),
            created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
            FOREIGN KEY (user_id) REFERENCES users(id)
        );
    `)
    if err != nil {
        panic(err)
    }

    // Insert sample data
    _, err = db.Exec(`
        INSERT OR IGNORE INTO users (name, email) VALUES
        ('Alice Smith', 'alice@example.com'),
        ('Bob Jones', 'bob@example.com'),
        ('Carol White', 'carol@example.com');

        INSERT OR IGNORE INTO orders (user_id, product, price) VALUES
        (1, 'Laptop', 999.99),
        (1, 'Mouse', 29.99),
        (2, 'Keyboard', 89.99),
        (3, 'Monitor', 299.99);
    `)
    if err != nil {
        panic(err)
    }
}
