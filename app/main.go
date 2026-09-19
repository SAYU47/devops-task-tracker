package main

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
	"log"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"
)

func readPassword() (string, error) {
	path := os.Getenv("DB_PASSWORD_FILE")

	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(string(data)), nil
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		next.ServeHTTP(w, r)

		if r.URL.Path != "/health" {
			slog.Info(
				"http request",
				"method", r.Method,
				"path", r.URL.Path,
				"remote_address", r.RemoteAddr,
				"duration_ms", time.Since(start).Milliseconds(),
			)
		}
	})
}

func main() {
	slog.SetDefault(
		slog.New(slog.NewJSONHandler(os.Stdout, nil)),
	)

	password, err := readPassword()
	if err != nil {
		log.Fatal("Cannot read database password: ", err)
	}

	databaseURL := fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=disable",
		os.Getenv("DB_USER"),
		password,
		os.Getenv("DB_HOST"),
		os.Getenv("DB_PORT"),
		os.Getenv("DB_NAME"),
	)

	db, err := pgxpool.New(context.Background(), databaseURL)
	if err != nil {
		log.Fatal("Cannot create database pool: ", err)
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.Ping(ctx); err != nil {
		log.Fatal("Cannot connect to database: ", err)
	}

	log.Println("Connected to PostgreSQL")

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}

		fmt.Fprintln(w, "DevOps Task Tracker")
	})

	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()

		w.Header().Set("Content-Type", "application/json")

		if err := db.Ping(ctx); err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			fmt.Fprintln(w, `{"status":"unhealthy","database":"unavailable"}`)
			return
		}

		fmt.Fprintln(w, `{"status":"healthy","database":"connected"}`)
	})

	http.HandleFunc("/api/tasks", tasksHandler(db))
	log.Println("Server started on port 8080")
	slog.Info("server started", "port", 8080)
	log.Fatal(http.ListenAndServe(":8080", loggingMiddleware(http.DefaultServeMux)))
}
