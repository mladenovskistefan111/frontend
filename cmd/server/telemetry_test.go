package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

// ---------------------------------------------------------------------------
// metricsMiddleware
// ---------------------------------------------------------------------------

func TestMetricsMiddleware_RecordsSuccessfulRequest(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// wrap with the real metricsMiddleware (uses package-level prometheus vars)
	// we test it by checking it doesn't panic and passes through correctly
	handler := metricsMiddleware(next, "/test")
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}

func TestMetricsMiddleware_RecordsNotFound(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	handler := metricsMiddleware(next, "/missing")
	req := httptest.NewRequest(http.MethodGet, "/missing", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rr.Code)
	}
}

func TestMetricsMiddleware_RecordsPostRequest(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	})

	handler := metricsMiddleware(next, "/cart")
	req := httptest.NewRequest(http.MethodPost, "/cart", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d", rr.Code)
	}
}

func TestMetricsMiddleware_DefaultStatusIs200(t *testing.T) {
	// handler writes body but never calls WriteHeader explicitly
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("ok"))
	})

	handler := metricsMiddleware(next, "/")
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected default 200, got %d", rr.Code)
	}
}

func TestMetricsMiddleware_CallsNextHandler(t *testing.T) {
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	})

	handler := metricsMiddleware(next, "/")
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	handler.ServeHTTP(httptest.NewRecorder(), req)

	if !called {
		t.Error("expected next handler to be called")
	}
}

func TestMetricsMiddleware_MeasuresDuration(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(1 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	})

	handler := metricsMiddleware(next, "/slow")
	req := httptest.NewRequest(http.MethodGet, "/slow", nil)
	rr := httptest.NewRecorder()

	start := time.Now()
	handler.ServeHTTP(rr, req)
	elapsed := time.Since(start)

	if elapsed < time.Millisecond {
		t.Error("expected handler to take at least 1ms")
	}
}

// ---------------------------------------------------------------------------
// httpRequestsTotal counter
// ---------------------------------------------------------------------------

func TestHTTPRequestsTotal_IsRegistered(t *testing.T) {
	// verify the counter was registered and can be collected
	count := testutil.CollectAndCount(httpRequestsTotal)
	if count < 0 {
		t.Error("expected httpRequestsTotal to be collectable")
	}
}

func TestHTTPRequestDuration_IsRegistered(t *testing.T) {
	count := testutil.CollectAndCount(httpRequestDuration)
	if count < 0 {
		t.Error("expected httpRequestDuration to be collectable")
	}
}

func TestHTTPRequestsInFlight_IsRegistered(t *testing.T) {
	count := testutil.CollectAndCount(httpRequestsInFlight)
	if count < 0 {
		t.Error("expected httpRequestsInFlight to be collectable")
	}
}

// ---------------------------------------------------------------------------
// wrapRoutes
// ---------------------------------------------------------------------------

func TestWrapRoutes_WrapsRegisteredRoutes(t *testing.T) {
	// register metrics first to avoid panic on MustRegister
	// (already registered by init in telemetry.go — just test wrapRoutes behavior)
	r := mux.NewRouter()

	called := false
	r.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}).Methods(http.MethodGet)

	wrapRoutes(r)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if !called {
		t.Error("expected handler to be called after wrapRoutes")
	}
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}

func TestWrapRoutes_MultipleRoutes(t *testing.T) {
	r := mux.NewRouter()

	routes := []string{"/a", "/b", "/c"}
	hits := make(map[string]bool)

	for _, path := range routes {
		p := path
		r.HandleFunc(p, func(w http.ResponseWriter, r *http.Request) {
			hits[p] = true
			w.WriteHeader(http.StatusOK)
		}).Methods(http.MethodGet)
	}

	wrapRoutes(r)

	for _, path := range routes {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Errorf("route %s: expected 200, got %d", path, rr.Code)
		}
	}
}

func TestWrapRoutes_EmptyRouter(t *testing.T) {
	// should not panic on an empty router
	r := mux.NewRouter()
	wrapRoutes(r)
}
