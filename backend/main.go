package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// func ReferrerPolicyMiddleware() gin.HandlerFunc {
//     return func(c *gin.Context) {
//         c.Writer.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
//         c.Next()
//     }
// }

func metricsMiddleware(ms *MetricsService) gin.HandlerFunc {
	return func(c *gin.Context) {

		start := time.Now()

		c.Next()

		duration := time.Since(start).Seconds()

		path := c.FullPath()
		if path == "" {
			path = c.Request.URL.Path
		}
		method := c.Request.Method

		// total requests
		ms.IncrementTotalRequests(method, path)

		// request duration
		ms.ObserveRequestDuration(duration, method, path)

		// request errors
		status := c.Writer.Status()
		if status >= 500 {
			ms.IncrementFailedRequests(method, path, strconv.Itoa(c.Writer.Status()))
		}
	}
}

func main() {
	log.SetOutput(os.Stderr)
	if os.Getenv("DEBUG") == "true" {
		logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
		slog.SetDefault(logger)
	}
	if os.Getenv("MEMORY_LEAK_MAX_MEMORY") != "" {
		go func() { memoryLeak(0, 0) }()
	}

	// monitoring
	ms := NewMetricsService()
	metricsHandler := promhttp.HandlerFor(ms.Registry(), promhttp.HandlerOpts{})
	defaultMetricsHandler := promhttp.Handler()

	// Server
	log.Println("Starting server...")
	router := gin.New()
	router.Use(cors.New(cors.Config{
		AllowOrigins: []string{"*"},
		AllowHeaders: []string{"*"},
		//         ExposeHeaders:    []string{"Content-Length", "Access-Control-Allow-Origin", "Access-Control-Allow-Credentials", "Access-Control-Allow-Headers", "Access-Control-Allow-Methods"},
	}))
	router.Use(metricsMiddleware(ms)) // request metrics are updated here
	router.GET("/metrics", gin.WrapH(metricsHandler))
	router.GET("/default-metrics", gin.WrapH(defaultMetricsHandler))
	router.GET("/fibonacci", fibonacciHandler)
	router.POST("/video", videoPostHandler)
	router.GET("/videos", videosGetHandler)
	router.GET("/ping", pingHandler)
	router.GET("/memory-leak", memoryLeakHandler)
	router.GET("/", rootHandler)
	port := os.Getenv("PORT")
	if len(port) == 0 {
		port = "8080"
	}
	server := &http.Server{
		Addr:    fmt.Sprintf(":%s", port),
		Handler: router.Handler(),
	}

	// Signals
	if len(os.Getenv("NO_SIGNALS")) > 0 {
		if err := server.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("HTTP server error: %v", err)
		}
	} else {
		go func() {
			if err := server.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
				log.Fatalf("HTTP server error: %v", err)
			}
			log.Println("Stopped serving new connections.")
		}()
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan
		shutdownCtx, shutdownRelease := context.WithTimeout(context.Background(), 60*time.Second)
		defer shutdownRelease()
		if err := server.Shutdown(shutdownCtx); err != nil {
			log.Fatalf("HTTP shutdown error: %v", err)
		}
		log.Println("Graceful shutdown complete.")
	}
}

func httpErrorBadRequest(err error, ctx *gin.Context) {
	httpError(err, ctx, http.StatusBadRequest)
}

func httpErrorInternalServerError(err error, ctx *gin.Context) {
	httpError(err, ctx, http.StatusInternalServerError)
}

func httpError(err error, ctx *gin.Context, status int) {
	log.Println(err.Error())
	ctx.String(status, err.Error())
}
