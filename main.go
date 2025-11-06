package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net"
	"net/http"
	"os"
	"runtime"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

var tmpl = template.Must(template.ParseFiles("templates/index.html"))

type Server struct {
	startTime time.Time
	reqTotal  uint64

	// sliding window of per-second counts (last 60 seconds)
	mu      sync.Mutex
	buckets [60]uint64
	idx     int
}

func NewServer() *Server {
	s := &Server{startTime: time.Now()}
	go s.rotateLoop()
	return s
}

func (s *Server) rotateLoop() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		s.mu.Lock()
		s.idx = (s.idx + 1) % len(s.buckets)
		s.buckets[s.idx] = 0
		s.mu.Unlock()
	}
}

func (s *Server) recordRequest() {
	atomic.AddUint64(&s.reqTotal, 1)
	s.mu.Lock()
	s.buckets[s.idx]++
	s.mu.Unlock()
}

func (s *Server) requestsPerMinute() uint64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	var sum uint64
	for _, v := range s.buckets {
		sum += v
	}
	return sum
}

func (s *Server) dashboardHandler(w http.ResponseWriter, r *http.Request) {
	s.recordRequest()

	hostname, _ := os.Hostname()
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	// copy buckets under lock and compute bar heights for the template
	s.mu.Lock()
	buckets := make([]uint64, len(s.buckets))
	copy(buckets, s.buckets[:])
	s.mu.Unlock()

	// compute heights (pixels) for a small sparkline; cap to 120px
	heights := make([]int, len(buckets))
	max := uint64(1)
	for _, v := range buckets {
		if v > max {
			max = v
		}
	}
	scale := 1.0
	if max > 0 {
		// scale to max 120px
		scale = 120.0 / float64(max)
	}
	for i, v := range buckets {
		h := int(float64(v) * scale)
		if h < 2 {
			h = 2
		}
		heights[i] = h
	}

	// client info (respect X-Forwarded-For / X-Real-IP when behind proxies/ingress)
	clientIP := func(r *http.Request) string {
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			parts := strings.Split(xff, ",")
			return strings.TrimSpace(parts[0])
		}
		if xr := r.Header.Get("X-Real-IP"); xr != "" {
			return xr
		}
		host, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			return r.RemoteAddr
		}
		return host
	}(r)

	// capture a small set of headers to show (omit sensitive ones)
	hdrs := map[string]string{}
	allowed := []string{"Host", "User-Agent", "Accept", "Accept-Language", "Referer", "X-Forwarded-For", "X-Real-IP"}
	for _, k := range allowed {
		if v := r.Header.Get(k); v != "" {
			hdrs[k] = v
		}
	}

	data := map[string]interface{}{
		"StartTime":      s.startTime.Format(time.RFC3339),
		"Uptime":         time.Since(s.startTime).Truncate(time.Second).String(),
		"RequestsTotal":  atomic.LoadUint64(&s.reqTotal),
		"RequestsPerMin": s.requestsPerMinute(),
		"Hostname":       hostname,
		"GoVersion":      runtime.Version(),
		"NumGoroutine":   runtime.NumGoroutine(),
		"MemAllocMB":     float64(m.Alloc) / 1024.0 / 1024.0,
		"BarHeights":     heights,
		// client/request info for the dashboard
		"ClientIP":      clientIP,
		"ClientMethod":  r.Method,
		"ClientPath":    r.URL.Path,
		"ClientUA":      r.UserAgent(),
		"ClientHeaders": hdrs,
	}

	if err := tmpl.Execute(w, data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) healthHandler(w http.ResponseWriter, r *http.Request) {
	s.recordRequest()
	w.Header().Set("Content-Type", "application/json")
	out := map[string]string{
		"status": "ok",
		"uptime": time.Since(s.startTime).Truncate(time.Second).String(),
	}
	_ = json.NewEncoder(w).Encode(out)
}

func (s *Server) metricsHandler(w http.ResponseWriter, r *http.Request) {
	s.recordRequest()
	w.Header().Set("Content-Type", "text/plain; version=0.0.4")
	uptime := time.Since(s.startTime).Seconds()
	fmt.Fprintf(w, "uptime_seconds %d\n", int64(uptime))
	fmt.Fprintf(w, "requests_total %d\n", atomic.LoadUint64(&s.reqTotal))
	fmt.Fprintf(w, "requests_per_min %d\n", s.requestsPerMinute())
}

func main() {
	// If built with modules that disable symbol table info, enable it for stack traces.
	_ = debug.SetGCPercent(100)

	port := 8080
	if p := os.Getenv("PORT"); p != "" {
		if v, err := strconv.Atoi(p); err == nil {
			port = v
		}
	}

	srv := NewServer()

	mux := http.NewServeMux()
	mux.HandleFunc("/", srv.dashboardHandler)
	mux.HandleFunc("/health", srv.healthHandler)
	mux.HandleFunc("/metrics", srv.metricsHandler)

	// serve static assets
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	addr := fmt.Sprintf(":%d", port)
	log.Printf("starting deepSight on %s\n", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}
