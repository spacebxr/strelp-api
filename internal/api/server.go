package api

import (
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/spacebxr/strelp/internal/cache"
)

type Server struct {
	Cache  *cache.Cache
	Router *chi.Mux
}

func NewServer(cache *cache.Cache) *Server {
	s := &Server{
		Cache:  cache,
		Router: chi.NewRouter(),
	}

	s.setupMiddleware()
	s.setupRoutes()

	return s
}

func (s *Server) setupMiddleware() {
	s.Router.Use(middleware.RequestID)
	s.Router.Use(middleware.RealIP)
	s.Router.Use(middleware.Logger)
	s.Router.Use(middleware.Recoverer)
	s.Router.Use(middleware.Timeout(60 * time.Second))

	s.Router.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Content-Type", "Content-Length", "Accept-Encoding", "X-CSRF-Token", "Authorization"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	}))
}

func (s *Server) setupRoutes() {
	s.Router.Get("/", s.handleIndex)
	s.Router.Get("/health", s.handleHealth)

	s.Router.Route("/v1", func(r chi.Router) {
		r.Route("/presence", func(r chi.Router) {
			r.Get("/{userID}", s.handleGetPresence)
			r.Get("/{userID}/ws", s.handleStreamPresence)
		})
	})
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"name": "Strelp Presence API", "version": "1.0.0"}`))
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"status": "ok"}`))
}

func (s *Server) Start(addr string) error {
	log.Printf("Starting API server on %s", addr)
	return http.ListenAndServe(addr, s.Router)
}
