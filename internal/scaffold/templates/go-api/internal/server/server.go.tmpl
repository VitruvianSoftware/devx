package server

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
)

// New builds and returns the application HTTP router.
func New(db *pgxpool.Pool) http.Handler {
	r := chi.NewRouter()

	// Middleware stack
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.CleanPath)

	// Routes
	r.Get("/healthz", handleHealth(db))

	// API v1
	r.Route("/api/v1", func(r chi.Router) {
		// Add your routes here:
		// r.Get("/items", handleListItems(db))
		// r.Post("/items", handleCreateItem(db))
	})

	return r
}
