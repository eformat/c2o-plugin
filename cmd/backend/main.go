package main

import (
	"crypto/tls"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gorilla/mux"
	"github.com/rhai-code/c2o-plugin/pkg/handlers"
)

func main() {
	port := getEnv("PORT", "9443")
	distDir := getEnv("PLUGIN_DIST_DIR", "dist")
	devMode := getEnv("DEV_MODE", "") == "true"
	certFile := getEnv("TLS_CERT_FILE", "/var/serving-cert/tls.crt")
	keyFile := getEnv("TLS_KEY_FILE", "/var/serving-cert/tls.key")

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	r := mux.NewRouter()

	// API routes with rate limiting and auth middleware
	api := r.PathPrefix("/api").Subrouter()
	api.Use(handlers.RateLimitMiddleware)
	api.Use(handlers.AuthMiddleware)

	api.HandleFunc("/namespaces", handlers.ListNamespaces).Methods("GET")
	api.HandleFunc("/namespaces", handlers.CreateNamespace).Methods("POST")
	api.HandleFunc("/agents", handlers.ListAgents).Methods("GET")
	api.HandleFunc("/agents/add", handlers.AddAgent).Methods("POST")
	api.HandleFunc("/agents/{name}", handlers.DeleteAgent).Methods("DELETE")
	api.HandleFunc("/agents/{name}/scale", handlers.ScaleAgent).Methods("PATCH")
	api.HandleFunc("/agents/{name}/pod", handlers.GetAgentPod).Methods("GET")
	api.HandleFunc("/agents/{name}/make-supervisor", handlers.MakeSupervisor).Methods("POST")
	api.HandleFunc("/deploy", handlers.Deploy).Methods("POST")
	api.HandleFunc("/credentials", handlers.CreateCredentials).Methods("POST")
	api.HandleFunc("/credentials", handlers.ListCredentials).Methods("GET")
	api.HandleFunc("/connection", handlers.GetConnection).Methods("GET")

	// Health check (no auth)
	r.HandleFunc("/api/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok"}`))
	}).Methods("GET")

	// Plugin manifest
	r.HandleFunc("/plugin-manifest.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		http.ServeFile(w, r, filepath.Join(distDir, "plugin-manifest.json"))
	})

	// Static frontend files
	fs := http.FileServer(http.Dir(distDir))
	r.PathPrefix("/").Handler(fs)

	addr := fmt.Sprintf(":%s", port)
	slog.Info("starting c2o-plugin server", "addr", addr, "devMode", devMode)

	if devMode {
		slog.Info("running in dev mode (no TLS)")
		if err := http.ListenAndServe(addr, r); err != nil {
			slog.Error("server failed", "error", err)
			os.Exit(1)
		}
	} else {
		tlsCfg := &tls.Config{MinVersion: tls.VersionTLS12}
		srv := &http.Server{
			Addr:      addr,
			Handler:   r,
			TLSConfig: tlsCfg,
		}
		slog.Info("starting TLS server", "cert", certFile, "key", keyFile)
		if err := srv.ListenAndServeTLS(certFile, keyFile); err != nil {
			slog.Error("server failed", "error", err)
			os.Exit(1)
		}
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
