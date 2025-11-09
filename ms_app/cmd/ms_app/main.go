package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	_ "github.com/lib/pq"
)

var db *sql.DB

type User struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

type HealthResponse struct {
	Status     string `json:"status"`
	Database   bool   `json:"database"`
	Timestamp  string `json:"timestamp"`
	Hostname   string `json:"hostname"`
	DBHost     string `json:"db_host,omitempty"`
	RetryCount int    `json:"retry_count,omitempty"`
}

func initDB() error {
	var err error

	// Пробуем разные варианты подключения в порядке приоритета
	connectionAttempts := []string{
		os.Getenv("DATABASE_URL"), // сначала пробуем из переменной окружения
		// "postgres://user:password@haproxy:5433/testdb?sslmode=disable",         // через HAProxy
		// "postgres://user:password@postgres-master:5432/testdb?sslmode=disable", // напрямую к мастеру
		// "postgres://user:password@postgres-slave1:5432/testdb?sslmode=disable", // напрямую к слейву 1
		// "postgres://user:password@postgres-slave2:5432/testdb?sslmode=disable", // напрямую к слейву 2
		"postgres://user:password@localhost:5433/testdb?sslmode=disable", // локально
	}

	var successfulConnStr string
	var lastErr error

	for i, attemptConnStr := range connectionAttempts {
		if attemptConnStr == "" {
			continue
		}

		log.Printf("Attempt %d: trying to connect to %s", i+1, maskPassword(attemptConnStr))

		db, err = sql.Open("postgres", attemptConnStr)
		if err != nil {
			lastErr = fmt.Errorf("failed to open connection: %v", err)
			log.Printf("Connection attempt %d failed: %v", i+1, err)
			time.Sleep(3 * time.Second)
			continue
		}

		// Настройка пула соединений
		db.SetMaxOpenConns(25)
		db.SetMaxIdleConns(25)
		db.SetConnMaxLifetime(5 * time.Minute)

		// Пытаемся пинговать с таймаутом
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		err = db.PingContext(ctx)
		if err != nil {
			lastErr = fmt.Errorf("failed to ping database: %v", err)
			log.Printf("Ping attempt %d failed: %v", i+1, err)
			db.Close()
			db = nil
			time.Sleep(3 * time.Second)
			continue
		}

		successfulConnStr = attemptConnStr
		log.Printf("✅ Successfully connected to database using: %s", maskPassword(successfulConnStr))

		// Определяем к какому хосту подключились
		if strings.Contains(attemptConnStr, "haproxy") {
			log.Println("Connected via HAProxy (load balancing)")
		} else if strings.Contains(attemptConnStr, "master") {
			log.Println("Connected directly to PostgreSQL Master")
		} else if strings.Contains(attemptConnStr, "slave") {
			log.Println("Connected directly to PostgreSQL Slave")
		} else {
			log.Println("Connected to database")
		}

		return nil
	}

	return fmt.Errorf("failed to connect to database after all attempts. Last error: %v", lastErr)
}

func maskPassword(connStr string) string {
	// Скрываем пароль в логах
	return strings.Replace(connStr, "password", "***", -1)
}

func createTable() error {
	if db == nil {
		return fmt.Errorf("database not initialized")
	}

	query := `
	CREATE TABLE IF NOT EXISTS users (
		id SERIAL PRIMARY KEY,
		name VARCHAR(100) NOT NULL,
		email VARCHAR(100) UNIQUE NOT NULL,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	)`

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := db.ExecContext(ctx, query)
	return err
}

func homeHandler(w http.ResponseWriter, r *http.Request) {
	hostname, _ := os.Hostname()

	html := `
<!DOCTYPE html>
<html>
<head>
    <title>Go PostgreSQL App</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 40px; }
        .info { background: #f5f5f5; padding: 20px; border-radius: 5px; }
        .links a { display: inline-block; margin: 10px; padding: 10px 20px; background: #007bff; color: white; text-decoration: none; border-radius: 5px; }
    </style>
</head>
<body>
    <h1>🚀 Go PostgreSQL Application</h1>
    <div class="info">
        <h3>Container Information:</h3>
        <p><strong>Hostname:</strong> %s</p>
        <p><strong>Time:</strong> %s</p>
        <p><strong>Database:</strong> %s</p>
    </div>
    <div class="links">
        <a href="/health">Health Check</a>
        <a href="/users">List Users</a>
        <a href="/users/create">Create User</a>
    </div>
    <div style="margin-top: 20px;">
        <h3>Test Database Connection:</h3>
        <form action="/users/create" method="POST">
            <input type="text" name="name" placeholder="Name" required style="padding: 8px; margin: 5px;">
            <input type="email" name="email" placeholder="Email" required style="padding: 8px; margin: 5px;">
            <button type="submit" style="padding: 8px 15px; margin: 5px; background: #28a745; color: white; border: none; border-radius: 3px;">Create User</button>
        </form>
    </div>
</body>
</html>
`

	dbStatus := "❌ Not connected"
	if db != nil {
		dbStatus = "✅ Connected via HAProxy"
	}

	fmt.Fprintf(w, html, hostname, time.Now().Format("2006-01-02 15:04:05"), dbStatus)
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	hostname, _ := os.Hostname()
	response := HealthResponse{
		Status:    "ok",
		Database:  false,
		Timestamp: time.Now().Format(time.RFC3339),
		Hostname:  hostname,
	}

	// Проверяем подключение к БД
	if db != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		err := db.PingContext(ctx)
		if err == nil {
			response.Database = true

			// Пытаемся определить к какому хосту подключены
			var host string
			err := db.QueryRowContext(ctx, "SELECT inet_server_addr()").Scan(&host)
			if err == nil {
				response.DBHost = host
			}
		} else {
			response.Status = "database_error"
			log.Printf("Database ping failed: %v", err)
		}
	} else {
		response.Status = "database_not_initialized"
	}

	w.Header().Set("Content-Type", "application/json")

	if response.Status != "ok" {
		w.WriteHeader(http.StatusServiceUnavailable)
	}

	json.NewEncoder(w).Encode(response)
}

func usersHandler(w http.ResponseWriter, r *http.Request) {
	if db == nil {
		http.Error(w, `{"error": "Database not connected"}`, http.StatusServiceUnavailable)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	rows, err := db.QueryContext(ctx, "SELECT id, name, email FROM users ORDER BY id")
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error": "Database query failed: %v"}`, err), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var users []User
	for rows.Next() {
		var user User
		if err := rows.Scan(&user.ID, &user.Name, &user.Email); err != nil {
			http.Error(w, fmt.Sprintf(`{"error": "Data scan failed: %v"}`, err), http.StatusInternalServerError)
			return
		}
		users = append(users, user)
	}

	if err = rows.Err(); err != nil {
		http.Error(w, fmt.Sprintf(`{"error": "Rows iteration failed: %v"}`, err), http.StatusInternalServerError)
		return
	}

	if users == nil {
		users = []User{} // Ensure empty array instead of null
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(users)
}

func createUserHandler(w http.ResponseWriter, r *http.Request) {
	if db == nil {
		http.Error(w, `{"error": "Database not connected"}`, http.StatusServiceUnavailable)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, `{"error": "Method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	name := r.FormValue("name")
	email := r.FormValue("email")

	if name == "" || email == "" {
		http.Error(w, `{"error": "Name and email are required"}`, http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var id int
	err := db.QueryRowContext(
		ctx,
		"INSERT INTO users (name, email) VALUES ($1, $2) RETURNING id",
		name, email,
	).Scan(&id)

	if err != nil {
		if strings.Contains(err.Error(), "unique constraint") {
			http.Error(w, `{"error": "Email already exists"}`, http.StatusConflict)
		} else {
			http.Error(w, fmt.Sprintf(`{"error": "Failed to create user: %v"}`, err), http.StatusInternalServerError)
		}
		return
	}

	response := map[string]interface{}{
		"id":      id,
		"name":    name,
		"email":   email,
		"message": "User created successfully",
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(response)
}

func main() {
	log.Println("🚀 Starting Go PostgreSQL Application...")
	log.Println("⏳ Waiting for dependencies to be ready...")

	// Даем время на запуск всех сервисов
	time.Sleep(10 * time.Second)

	// Инициализация БД с ретраями
	maxRetries := 12
	var retryCount int

	for i := 0; i < maxRetries; i++ {
		retryCount = i + 1
		log.Printf("🔧 Database connection attempt %d/%d", retryCount, maxRetries)

		err := initDB()
		if err == nil {
			break
		}

		log.Printf("❌ Database initialization failed (attempt %d): %v", retryCount, err)

		if i < maxRetries-1 {
			waitTime := time.Duration(i+1) * 5 * time.Second
			log.Printf("⏰ Waiting %v before next attempt...", waitTime)
			time.Sleep(waitTime)
		} else {
			log.Printf("💥 All database connection attempts failed after %d retries", maxRetries)
			log.Println("⚠️  Starting in degraded mode (without database)")
		}
	}

	// Пытаемся создать таблицу если БД подключена
	if db != nil {
		if err := createTable(); err != nil {
			log.Printf("⚠️  Could not create table: %v", err)
		} else {
			log.Println("✅ Database table checked/created successfully")
		}
	}

	// HTTP роуты
	http.HandleFunc("/", homeHandler)
	http.HandleFunc("/health", healthHandler)
	http.HandleFunc("/users", usersHandler)
	http.HandleFunc("/users/create", createUserHandler)

	port := os.Getenv("PORT")
	if port == "" {
		port = "3025"
	}

	log.Printf("🌐 Server starting on port %s", port)
	log.Printf("📊 Health check available at: http://0.0.0.0:%s/health", port)
	log.Printf("👥 Users API available at: http://0.0.0.0:%s/users", port)

	err := http.ListenAndServe(":"+port, nil)
	if err != nil {
		log.Fatalf("💥 Failed to start server: %v", err)
	}
}
