package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sirupsen/logrus"
)

// ---------------------------------------------------------------------------
// ensureSessionID
// ---------------------------------------------------------------------------

func TestEnsureSessionID_NewSession(t *testing.T) {
	// no cookie set — should create a new session ID and set a cookie
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		id := r.Context().Value(ctxKeySessionID{})
		if id == nil {
			t.Error("expected session ID in context, got nil")
		}
		sid, ok := id.(string)
		if !ok || sid == "" {
			t.Errorf("expected non-empty session ID string, got %v", id)
		}
	})

	handler := ensureSessionID(next)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if !called {
		t.Error("next handler was not called")
	}

	// cookie should be set in the response
	cookies := rr.Result().Cookies()
	found := false
	for _, c := range cookies {
		if c.Name == cookieSessionID {
			found = true
			if c.Value == "" {
				t.Error("session cookie value should not be empty")
			}
		}
	}
	if !found {
		t.Errorf("expected cookie %q to be set", cookieSessionID)
	}
}

func TestEnsureSessionID_ExistingSession(t *testing.T) {
	// cookie already set — should reuse existing session ID
	existingID := "existing-session-id-123"
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		id := r.Context().Value(ctxKeySessionID{})
		if id == nil {
			t.Fatal("expected session ID in context, got nil")
		}
		if id.(string) != existingID {
			t.Errorf("expected session ID %q, got %q", existingID, id.(string))
		}
	})

	handler := ensureSessionID(next)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: cookieSessionID, Value: existingID})
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if !called {
		t.Error("next handler was not called")
	}
}

func TestEnsureSessionID_SingleSharedSession(t *testing.T) {
	// ENABLE_SINGLE_SHARED_SESSION=true should use hardcoded session ID
	t.Setenv("ENABLE_SINGLE_SHARED_SESSION", "true")

	var capturedID string
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Context().Value(ctxKeySessionID{})
		if id != nil {
			capturedID = id.(string)
		}
	})

	handler := ensureSessionID(next)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	expected := "12345678-1234-1234-1234-123456789123"
	if capturedID != expected {
		t.Errorf("expected shared session ID %q, got %q", expected, capturedID)
	}
}

func TestEnsureSessionID_TwoRequestsGetDifferentIDs(t *testing.T) {
	// two requests without cookies should get different session IDs
	ids := make([]string, 2)
	for i := 0; i < 2; i++ {
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id := r.Context().Value(ctxKeySessionID{})
			if id != nil {
				ids[i] = id.(string)
			}
		})
		handler := ensureSessionID(next)
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
	}

	if ids[0] == ids[1] {
		t.Errorf("expected different session IDs for different requests, got same: %q", ids[0])
	}
}

// ---------------------------------------------------------------------------
// logHandler
// ---------------------------------------------------------------------------

func TestLogHandler_PassesRequestThrough(t *testing.T) {
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	log := logrus.New()
	log.SetOutput(httptest.NewRecorder()) // discard log output
	handler := &logHandler{log: log, next: next}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if !called {
		t.Error("next handler was not called")
	}
	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}
}

func TestLogHandler_InjectsLoggerIntoContext(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log := r.Context().Value(ctxKeyLog{})
		if log == nil {
			t.Error("expected logger in context, got nil")
		}
	})

	log := logrus.New()
	handler := &logHandler{log: log, next: next}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
}

func TestLogHandler_InjectsRequestIDIntoContext(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Context().Value(ctxKeyRequestID{})
		if id == nil {
			t.Error("expected request ID in context, got nil")
		}
		if id.(string) == "" {
			t.Error("expected non-empty request ID")
		}
	})

	log := logrus.New()
	handler := &logHandler{log: log, next: next}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
}

func TestLogHandler_RecordsStatusCode(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	log := logrus.New()
	handler := &logHandler{log: log, next: next}

	req := httptest.NewRequest(http.MethodGet, "/missing", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", rr.Code)
	}
}

// ---------------------------------------------------------------------------
// responseRecorder
// ---------------------------------------------------------------------------

func TestResponseRecorder_DefaultStatus(t *testing.T) {
	rr := &responseRecorder{w: httptest.NewRecorder()}
	// write without setting status — should default to 200
	_, _ = rr.Write([]byte("hello"))
	if rr.status != http.StatusOK {
		t.Errorf("expected default status 200, got %d", rr.status)
	}
}

func TestResponseRecorder_ExplicitStatus(t *testing.T) {
	rr := &responseRecorder{w: httptest.NewRecorder()}
	rr.WriteHeader(http.StatusCreated)
	if rr.status != http.StatusCreated {
		t.Errorf("expected status 201, got %d", rr.status)
	}
}

func TestResponseRecorder_ByteCount(t *testing.T) {
	rr := &responseRecorder{w: httptest.NewRecorder()}
	body := []byte("hello world")
	_, _ = rr.Write(body)
	if rr.b != len(body) {
		t.Errorf("expected %d bytes recorded, got %d", len(body), rr.b)
	}
}

func TestResponseRecorder_Header(t *testing.T) {
	inner := httptest.NewRecorder()
	rr := &responseRecorder{w: inner}
	rr.Header().Set("Content-Type", "application/json")
	if inner.Header().Get("Content-Type") != "application/json" {
		t.Error("expected Content-Type header to be set on inner writer")
	}
}
