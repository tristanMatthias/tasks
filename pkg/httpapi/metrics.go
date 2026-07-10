package httpapi

import (
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

// metrics is a tiny, dependency-free Prometheus-style collector.
type metrics struct {
	start    time.Time
	inFlight atomic.Int64
	total    atomic.Int64

	mu       sync.Mutex
	byStatus map[int]int64 // status class bucket (2xx/4xx/5xx) counts by exact code
}

func newMetrics(now time.Time) *metrics {
	return &metrics{start: now, byStatus: map[int]int64{}}
}

func (m *metrics) middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		m.inFlight.Add(1)
		defer m.inFlight.Add(-1)
		m.total.Add(1)
		rec := &responseRecorder{ResponseWriter: w}
		next.ServeHTTP(rec, r)
		if rec.status == 0 {
			rec.status = http.StatusOK
		}
		m.mu.Lock()
		m.byStatus[rec.status]++
		m.mu.Unlock()
	})
}

// handler renders Prometheus text exposition format.
func (m *metrics) handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		m.mu.Lock()
		byStatus := make(map[int]int64, len(m.byStatus))
		for k, v := range m.byStatus {
			byStatus[k] = v
		}
		m.mu.Unlock()

		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
		fmt.Fprintf(w, "# HELP tasks_uptime_seconds Server uptime in seconds.\n# TYPE tasks_uptime_seconds gauge\ntasks_uptime_seconds %.0f\n", time.Since(m.start).Seconds())
		fmt.Fprintf(w, "# HELP tasks_requests_total Total HTTP requests handled.\n# TYPE tasks_requests_total counter\ntasks_requests_total %d\n", m.total.Load())
		fmt.Fprintf(w, "# HELP tasks_requests_in_flight In-flight HTTP requests.\n# TYPE tasks_requests_in_flight gauge\ntasks_requests_in_flight %d\n", m.inFlight.Load())
		fmt.Fprintf(w, "# HELP tasks_responses_total HTTP responses by status code.\n# TYPE tasks_responses_total counter\n")
		for code, n := range byStatus {
			fmt.Fprintf(w, "tasks_responses_total{code=\"%d\"} %d\n", code, n)
		}
	}
}
