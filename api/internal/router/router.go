package router

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/caze/ascend/api/internal/auth"
	"github.com/caze/ascend/api/internal/handler"
	appmw "github.com/caze/ascend/api/internal/middleware"
	"github.com/caze/ascend/api/internal/store"
)

var allowedOrigins = map[string]struct{}{
	"http://localhost:5173": {},
	"http://localhost:5174": {},
}

func New(s *store.Store, j *auth.JWT, rl *appmw.RateLimiter) chi.Router {
	r := chi.NewRouter()
	r.Use(cors)
	r.Use(middleware.RequestID)
	r.Use(requestLogger)
	r.Get("/healthz", handler.Health)
	r.Route("/api/v1", func(r chi.Router) {
		ah := handler.NewAuthHandler(s, j)
		r.Route("/auth", ah.Routes)
		ch := handler.NewChallengesHandler(s)
		teacherOnly := auth.RequireRole("teacher")
		r.Route("/challenges", func(r chi.Router) {
			ch.Routes(r)
			r.With(j.OptionalMiddleware).Get("/", ch.List)
			r.With(j.Middleware).Get("/{id}/stats", ch.Stats)
			r.With(j.Middleware, rl.Handler).Post("/{id}/submissions", ch.CreateSubmission)
			r.Group(func(r chi.Router) {
				r.Use(j.Middleware, teacherOnly)
				r.Post("/", ch.Create)
				r.Put("/{id}", ch.Update)
				r.Delete("/{id}", ch.Delete)
				r.Post("/{id}/test-cases", ch.CreateTestCase)
				r.Put("/{id}/test-cases", ch.ReplaceTestCases)
				r.Get("/{id}/test-cases", ch.ListTestCases)
			})
		})
		r.With(j.Middleware).Get("/submissions", ch.ListMySubmissions)
		r.Get("/submissions/{id}", ch.GetSubmission)
		r.With(j.Middleware, teacherOnly).Get("/classes/scoreboard", ch.TeacherScoreboard)

		lh := handler.NewListsHandler(s)
		r.Route("/lists", func(r chi.Router) {
			// Public: anyone can browse published lists, no accounts
			// required. Auth is optional here (OptionalMiddleware) so an
			// authenticated teacher additionally sees their own drafts.
			r.With(j.OptionalMiddleware).Get("/", lh.List)
			r.With(j.OptionalMiddleware).Get("/{id}", lh.Get)
			r.Group(func(r chi.Router) {
				r.Use(j.Middleware, teacherOnly)
				r.Post("/", lh.Create)
				r.Post("/import", lh.Import)
				r.Patch("/{id}", lh.Update)
				r.Delete("/{id}", lh.Delete)
				r.Post("/{id}/items", lh.CreateItem)
				r.Patch("/{id}/reorder", lh.Reorder)
			})
		})
		r.Route("/list-items", func(r chi.Router) {
			r.Use(j.Middleware, teacherOnly)
			r.Patch("/{id}", lh.UpdateItem)
			r.Delete("/{id}", lh.DeleteItem)
			// Completion (POST/DELETE /{id}/complete) is disabled for now:
			// without real student accounts there's no identity to attach a
			// completion to. Re-add once accounts are back.
		})
	})
	return r
}

func cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if _, ok := allowedOrigins[origin]; ok {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		}

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func requestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		next.ServeHTTP(ww, r)
		slog.InfoContext(r.Context(), "request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", ww.Status(),
			"latency_ms", time.Since(start).Milliseconds(),
			"request_id", middleware.GetReqID(r.Context()),
		)
	})
}
