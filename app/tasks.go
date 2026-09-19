package main

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Task struct {
	ID        int64     `json:"id"`
	Title     string    `json:"title"`
	Completed bool      `json:"completed"`
	CreatedAt time.Time `json:"created_at"`
}

type CreateTaskRequest struct {
	Title string `json:"title"`
}

func tasksHandler(db *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			getTasks(w, r, db)

		case http.MethodPost:
			createTask(w, r, db)

		default:
			w.Header().Set("Allow", "GET, POST")
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}
}

func getTasks(w http.ResponseWriter, r *http.Request, db *pgxpool.Pool) {
	rows, err := db.Query(
		r.Context(),
		"SELECT id, title, completed, created_at FROM tasks ORDER BY id",
	)
	if err != nil {
		http.Error(w, "Cannot read tasks", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	tasks := make([]Task, 0)

	for rows.Next() {
		var task Task

		if err := rows.Scan(
			&task.ID,
			&task.Title,
			&task.Completed,
			&task.CreatedAt,
		); err != nil {
			http.Error(w, "Cannot scan task", http.StatusInternalServerError)
			return
		}

		tasks = append(tasks, task)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tasks)
}

func createTask(w http.ResponseWriter, r *http.Request, db *pgxpool.Pool) {
	var request CreateTaskRequest

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	request.Title = strings.TrimSpace(request.Title)

	if request.Title == "" {
		http.Error(w, "Title is required", http.StatusBadRequest)
		return
	}

	var task Task

	err := db.QueryRow(
		r.Context(),
		`INSERT INTO tasks (title)
		 VALUES ($1)
		 RETURNING id, title, completed, created_at`,
		request.Title,
	).Scan(
		&task.ID,
		&task.Title,
		&task.Completed,
		&task.CreatedAt,
	)

	if err != nil {
		http.Error(w, "Cannot create task", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(task)
}
