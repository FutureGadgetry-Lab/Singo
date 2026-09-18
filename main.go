package main

import (
	"context"
	"embed"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"
)

//go:embed web/*
var webFiles embed.FS

type config struct {
	Listen   string
	GRPC     string
	Data     string
	Interval time.Duration
	KeepDays int
	User     string
	Password string
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func loadConfig() config {
	interval, _ := time.ParseDuration(env("SINGO_INTERVAL", "10s"))
	keepDays, _ := strconv.Atoi(env("SINGO_KEEP_DAYS", "90"))
	c := config{Interval: interval, KeepDays: keepDays}
	flag.StringVar(&c.Listen, "listen", env("SINGO_LISTEN", "127.0.0.1:8088"), "HTTP listen address")
	flag.StringVar(&c.GRPC, "grpc", env("SINGO_GRPC", "127.0.0.1:8080"), "sing-box V2Ray API address")
	flag.StringVar(&c.Data, "data", env("SINGO_DATA", "./singo-data.json"), "state file path")
	flag.DurationVar(&c.Interval, "interval", interval, "collection interval")
	flag.IntVar(&c.KeepDays, "keep-days", keepDays, "daily history retention")
	flag.StringVar(&c.User, "user", env("SINGO_USER", ""), "dashboard username")
	flag.StringVar(&c.Password, "password", env("SINGO_PASSWORD", ""), "dashboard password")
	flag.Parse()
	if c.Interval < time.Second {
		c.Interval = time.Second
	}
	if c.KeepDays < 1 {
		c.KeepDays = 1
	}
	return c
}

func main() {
	cfg := loadConfig()
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	if err := validateConfig(cfg); err != nil {
		logger.Error("配置无效", "error", err)
		os.Exit(2)
	}
	store, err := openStore(cfg.Data, cfg.KeepDays)
	if err != nil {
		logger.Error("读取状态文件失败", "error", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	collector := newCollector(cfg.GRPC, cfg.Interval, store, logger)
	go collector.run(ctx)

	assets, _ := fs.Sub(webFiles, "web")
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/overview", func(w http.ResponseWriter, r *http.Request) { writeJSON(w, store.overview(collector.status())) })
	mux.HandleFunc("GET /api/history", func(w http.ResponseWriter, r *http.Request) { writeJSON(w, store.history()) })
	mux.Handle("GET /", http.FileServer(http.FS(assets)))
	handler := securityHeaders(basicAuth(cfg.User, cfg.Password, mux))
	server := &http.Server{Addr: cfg.Listen, Handler: handler, ReadHeaderTimeout: 5 * time.Second, IdleTimeout: 60 * time.Second}

	go func() {
		logger.Info("Singo 已启动", "listen", cfg.Listen, "sing_box", cfg.GRPC, "data", cfg.Data)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("HTTP 服务退出", "error", err)
			stop()
		}
	}()
	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = server.Shutdown(shutdownCtx)
	if err := store.save(); err != nil {
		logger.Error("保存状态失败", "error", err)
	}
}

func writeJSON(w http.ResponseWriter, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	if err := json.NewEncoder(w).Encode(value); err != nil {
		http.Error(w, err.Error(), 500)
	}
}

func basicAuth(user, pass string, next http.Handler) http.Handler {
	if user == "" && pass == "" {
		return next
	}
	want := "Basic " + base64.StdEncoding.EncodeToString([]byte(user+":"+pass))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if subtleEqual(r.Header.Get("Authorization"), want) == false {
			w.Header().Set("WWW-Authenticate", `Basic realm="Singo"`)
			http.Error(w, "需要登录", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func subtleEqual(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	var diff byte
	for i := range a {
		diff |= a[i] ^ b[i]
	}
	return diff == 0
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; style-src 'self' 'unsafe-inline'; script-src 'self'; connect-src 'self'")
		next.ServeHTTP(w, r)
	})
}

func parseStatName(name string) (user, direction string, ok bool) {
	parts := strings.Split(name, ">>>")
	if len(parts) != 4 || parts[0] != "user" || parts[2] != "traffic" {
		return "", "", false
	}
	if parts[3] != "uplink" && parts[3] != "downlink" {
		return "", "", false
	}
	return parts[1], parts[3], parts[1] != ""
}

func validateConfig(c config) error {
	if c.Listen == "" || c.GRPC == "" || c.Data == "" {
		return fmt.Errorf("listen、grpc 和 data 不能为空")
	}
	return nil
}
