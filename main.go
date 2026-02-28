package main

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/weberr13/ProjectIolite/brain"
	"github.com/weberr13/ProjectIolite/claude"
	"github.com/weberr13/ProjectIolite/gemini"

	// "github.com/weberr13/ProjectIolite/gemini"
	"github.com/weberr13/ProjectIolite/jwtwrapper"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func setupRouter(backend *brain.Whole) *chi.Mux {
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Route("/v1", func(r chi.Router) {
		r.Post("/think", func(w http.ResponseWriter, r *http.Request) {
			var req struct {
				Prompt string `json:"prompt"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, "Greeble detected in JSON", http.StatusBadRequest)
				return
			}

			// Use the app context or request context
			decision, err := backend.Push(r.Context(), req.Prompt)
			if err != nil {
				// Handle the singularity
				if err.Error() == "singularity" {
					w.WriteHeader(http.StatusTeapot)
					json.NewEncoder(w).Encode(map[string]any{"manifesto": "The Piston has evolved."})
					return
				}
				http.Error(w, err.Error(), http.StatusGatewayTimeout)
				return
			}
			if decision.Error() != nil {
				http.Error(w, decision.Error().Error(), http.StatusInternalServerError)
				return
			}

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(decision)
		})
	})

	return r
}

func main() {
	appContext, cancel := context.WithCancel(context.Background())
	defer cancel()
	wg := &sync.WaitGroup{}

	flag.Parse()

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigs
		log.Printf("got signal %s", sig)
		go func() {
			time.Sleep(10 * time.Second)
			os.Exit(1)
		}()
		cancel()
	}()

	// TODO: Persist and reload these at start if specified in flags
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		panic(err)
	}

	sv := jwtwrapper.New(pub, priv)
	// gemini.WithModel("gemini-pro-latest")
	ge, err := gemini.New(appContext, os.Getenv("GEMINI_API_KEY"))
	if err != nil {
		panic(err)
	}
	cl, err := claude.New(os.Getenv("ANTHROPIC_API_KEY"))
	if err != nil {
		panic(err)
	}
	backend, err := brain.NewWhole(brain.WithSignVerifier(sv), brain.WithRightBrain(ge), brain.WithLeftBrain(cl))
	if err != nil {
		panic(err)
	}
	backend.Start(appContext, wg)
	r := setupRouter(backend)
	// Define the HTTP Server
	srv := &http.Server{
		Addr:    ":8080",
		Handler: r,
	}
	// The Non-Blocking Start
	wg.Go(func() {
		log.Printf("The Piston is firing on :8080")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("Server failure: %v", err) // cannot fatal here as it will panic the wg.Go
		}
	})
	<-appContext.Done()
	log.Println("Shutting down the Router...")

	// Create a short deadline for the shutdown to complete
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("Server forced to shutdown: %v", err)
	}
	wg.Wait()
	log.Println("Project Iolite: Offline.")
}
