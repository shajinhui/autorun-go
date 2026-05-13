package main

import (
	"context"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	handler "autorun-go/api"
)

func main() {
	handler.StartLocalClubScheduler()

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/api", handler.Handler)
	mux.HandleFunc("/api/", handler.Handler)

	frontendDist := resolveFrontendDist()
	if frontendDist != "" {
		log.Printf("serving frontend from %s", frontendDist)
		mux.Handle("/", spaFileServer(frontendDist))
	} else {
		mux.HandleFunc("/", handler.Handler)
	}

	server := &http.Server{
		Handler: mux,
	}
	handler.SetLocalAppShutdownFunc(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if err := server.Shutdown(ctx); err != nil {
			log.Printf("local backend shutdown failed: %v", err)
		}
	})

	listener, err := net.Listen("tcp", ":"+port)
	if err != nil {
		log.Fatalf("server failed: %v", err)
	}
	addr := listener.Addr().String()
	log.Printf("autorun-go listening on %s", addr)
	if frontendDist != "" && shouldOpenBrowser() {
		openURL := "http://localhost:" + port
		go openBrowserSoon(openURL)
	}

	if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
		log.Fatalf("server failed: %v", err)
	}
}

func resolveFrontendDist() string {
	candidates := make([]string, 0, 3)
	if configured := strings.TrimSpace(os.Getenv("FRONTEND_DIST")); configured != "" {
		candidates = append(candidates, configured)
	}

	if exe, err := os.Executable(); err == nil {
		candidates = append(candidates, filepath.Join(filepath.Dir(exe), "web"))
	}
	if cwd, err := os.Getwd(); err == nil {
		candidates = append(candidates, filepath.Join(cwd, "web"))
	}

	for _, candidate := range candidates {
		abs, err := filepath.Abs(candidate)
		if err != nil {
			continue
		}
		if _, err := os.Stat(filepath.Join(abs, "index.html")); err == nil {
			return abs
		}
	}
	return ""
}

func spaFileServer(dist string) http.Handler {
	fileServer := http.FileServer(http.Dir(dist))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cleanPath := strings.TrimPrefix(path.Clean("/"+r.URL.Path), "/")
		requestedPath := filepath.Join(dist, cleanPath)
		if stat, err := os.Stat(requestedPath); err == nil && !stat.IsDir() {
			fileServer.ServeHTTP(w, r)
			return
		}

		http.ServeFile(w, r, filepath.Join(dist, "index.html"))
	})
}

func shouldOpenBrowser() bool {
	value := strings.ToLower(strings.TrimSpace(os.Getenv("AUTO_OPEN_BROWSER")))
	return value != "0" && value != "false" && value != "no"
}

func openBrowserSoon(url string) {
	time.Sleep(500 * time.Millisecond)
	if err := openBrowser(url); err != nil {
		log.Printf("open browser failed: %v", err)
	}
}

func openBrowser(url string) error {
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("open", url).Start()
	case "windows":
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	default:
		return exec.Command("xdg-open", url).Start()
	}
}
