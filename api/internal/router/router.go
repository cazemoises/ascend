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

func New(s *store.Store, pa *appmw.PangolinAuth, rl *appmw.RateLimiter) chi.Router {
	r := chi.NewRouter()
	r.Use(cors)
	r.Use(middleware.RequestID)
	r.Use(requestLogger)
	r.Get("/healthz", handler.Health)
	r.Route("/api/v1", func(r chi.Router) {
		// Resolves identity from Remote-Email for every route below; never
		// rejects by itself. Routes that require a signed-in caller add
		// auth.RequireAuthenticated.
		r.Use(pa.Middleware)
		// Lets a real teacher preview the app as a student (X-View-As), never
		// the other way around. Must run after PangolinAuth and before any
		// RequireRole check.
		r.Use(appmw.ViewAs)

		r.Get("/auth/me", handler.Me)

		ch := handler.NewChallengesHandler(s)
		teacherOnly := auth.RequireRole("teacher")
		r.Route("/challenges", func(r chi.Router) {
			ch.Routes(r)
			r.Get("/", ch.List)
			r.With(auth.RequireAuthenticated).Get("/{id}/stats", ch.Stats)
			r.With(auth.RequireAuthenticated, rl.Handler).Post("/{id}/submissions", ch.CreateSubmission)
			r.Group(func(r chi.Router) {
				r.Use(auth.RequireAuthenticated, teacherOnly)
				r.Post("/", ch.Create)
				r.Post("/import", ch.Import)
				r.Put("/{id}", ch.Update)
				r.Delete("/{id}", ch.Delete)
				r.Post("/{id}/test-cases", ch.CreateTestCase)
				r.Put("/{id}/test-cases", ch.ReplaceTestCases)
				r.Get("/{id}/test-cases", ch.ListTestCases)
			})
		})
		cch := handler.NewChallengeCollectionsHandler(s)
		r.Route("/challenge-collections", func(r chi.Router) {
			r.Group(func(r chi.Router) {
				r.Use(auth.RequireAuthenticated, teacherOnly)
				r.Get("/", cch.List)
				r.Post("/", cch.Create)
				r.Patch("/{id}", cch.Update)
				r.Delete("/{id}", cch.Delete)
			})
		})

		r.With(auth.RequireAuthenticated).Get("/submissions", ch.ListMySubmissions)
		r.Get("/submissions/{id}", ch.GetSubmission)

		th := handler.NewTeacherHandler(s)
		r.With(auth.RequireAuthenticated, teacherOnly).Get("/teacher/students-overview", th.StudentsOverview)

		lh := handler.NewListsHandler(s)
		r.Route("/lists", func(r chi.Router) {
			// Public: anyone can browse published lists, no accounts
			// required. An authenticated teacher additionally sees their own
			// drafts (PangolinAuth already ran above, so claims are present
			// whenever Pangolin/DEV_FAKE_EMAIL identified the caller).
			r.Get("/", lh.List)
			r.Get("/{id}", lh.Get)
			r.Group(func(r chi.Router) {
				r.Use(auth.RequireAuthenticated, teacherOnly)
				r.Post("/", lh.Create)
				r.Post("/import", lh.Import)
				r.Patch("/{id}", lh.Update)
				r.Delete("/{id}", lh.Delete)
				r.Post("/{id}/items", lh.CreateItem)
				r.Patch("/{id}/reorder", lh.Reorder)
			})
		})
		r.Route("/list-items", func(r chi.Router) {
			r.Group(func(r chi.Router) {
				r.Use(auth.RequireAuthenticated, teacherOnly)
				r.Patch("/{id}", lh.UpdateItem)
				r.Delete("/{id}", lh.DeleteItem)
			})
			// Any authenticated user can mark their own completion — Pangolin
			// now provisions a real account per caller, so claims.UserID always
			// identifies a specific student here.
			r.Group(func(r chi.Router) {
				r.Use(auth.RequireAuthenticated)
				r.Post("/{id}/complete", lh.CompleteItem)
				r.Delete("/{id}/complete", lh.UncompleteItem)
			})
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
