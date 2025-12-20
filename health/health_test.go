package health

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestChecker(t *testing.T) {
	t.Run("register and check", func(t *testing.T) {
		c := NewChecker()

		c.RegisterFunc("test", func(ctx context.Context) error {
			return nil
		}, true)

		result := c.Check(context.Background(), "test")

		if result.Status != StatusHealthy {
			t.Errorf("expected healthy, got %s", result.Status)
		}
	})

	t.Run("check unknown returns unknown status", func(t *testing.T) {
		c := NewChecker()

		result := c.Check(context.Background(), "nonexistent")

		if result.Status != StatusUnknown {
			t.Errorf("expected unknown, got %s", result.Status)
		}
	})

	t.Run("failing critical check returns unhealthy", func(t *testing.T) {
		c := NewChecker()

		c.RegisterFunc("critical", func(ctx context.Context) error {
			return errors.New("service down")
		}, true)

		result := c.Check(context.Background(), "critical")

		if result.Status != StatusUnhealthy {
			t.Errorf("expected unhealthy, got %s", result.Status)
		}
		if result.Message != "service down" {
			t.Errorf("expected message 'service down', got %s", result.Message)
		}
	})

	t.Run("failing non-critical check returns degraded", func(t *testing.T) {
		c := NewChecker()

		c.RegisterFunc("non-critical", func(ctx context.Context) error {
			return errors.New("cache unavailable")
		}, false)

		result := c.Check(context.Background(), "non-critical")

		if result.Status != StatusDegraded {
			t.Errorf("expected degraded, got %s", result.Status)
		}
	})

	t.Run("check timeout", func(t *testing.T) {
		c := NewChecker()

		c.Register(CheckConfig{
			Name: "slow",
			Check: func(ctx context.Context) error {
				select {
				case <-time.After(5 * time.Second):
					return nil
				case <-ctx.Done():
					return ctx.Err()
				}
			},
			Timeout:  50 * time.Millisecond,
			Critical: true,
		})

		result := c.Check(context.Background(), "slow")

		if result.Status != StatusUnhealthy {
			t.Errorf("expected unhealthy due to timeout, got %s", result.Status)
		}
	})

	t.Run("check all", func(t *testing.T) {
		c := NewChecker()

		c.RegisterFunc("check1", func(ctx context.Context) error {
			return nil
		}, true)
		c.RegisterFunc("check2", func(ctx context.Context) error {
			return nil
		}, true)

		results := c.CheckAll(context.Background())

		if len(results) != 2 {
			t.Errorf("expected 2 results, got %d", len(results))
		}

		for name, result := range results {
			if result.Status != StatusHealthy {
				t.Errorf("check %s: expected healthy, got %s", name, result.Status)
			}
		}
	})

	t.Run("overall status healthy", func(t *testing.T) {
		c := NewChecker()

		c.RegisterFunc("check1", func(ctx context.Context) error {
			return nil
		}, true)

		status, _ := c.OverallStatus(context.Background())

		if status != StatusHealthy {
			t.Errorf("expected healthy, got %s", status)
		}
	})

	t.Run("overall status unhealthy when critical check fails", func(t *testing.T) {
		c := NewChecker()

		c.RegisterFunc("healthy", func(ctx context.Context) error {
			return nil
		}, true)
		c.RegisterFunc("unhealthy", func(ctx context.Context) error {
			return errors.New("error")
		}, true)

		status, _ := c.OverallStatus(context.Background())

		if status != StatusUnhealthy {
			t.Errorf("expected unhealthy, got %s", status)
		}
	})

	t.Run("overall status degraded when non-critical check fails", func(t *testing.T) {
		c := NewChecker()

		c.RegisterFunc("healthy", func(ctx context.Context) error {
			return nil
		}, true)
		c.RegisterFunc("non-critical", func(ctx context.Context) error {
			return errors.New("warning")
		}, false)

		status, _ := c.OverallStatus(context.Background())

		if status != StatusDegraded {
			t.Errorf("expected degraded, got %s", status)
		}
	})

	t.Run("unregister", func(t *testing.T) {
		c := NewChecker()

		c.RegisterFunc("test", func(ctx context.Context) error {
			return nil
		}, true)

		c.Unregister("test")

		result := c.Check(context.Background(), "test")
		if result.Status != StatusUnknown {
			t.Errorf("expected unknown after unregister, got %s", result.Status)
		}
	})
}

func TestBackgroundChecker(t *testing.T) {
	t.Run("background checking", func(t *testing.T) {
		c := NewChecker()

		var checkCount atomic.Int32
		c.RegisterFunc("counter", func(ctx context.Context) error {
			checkCount.Add(1)
			return nil
		}, true)

		c.StartBackground(50 * time.Millisecond)
		time.Sleep(150 * time.Millisecond)
		c.StopBackground()

		count := checkCount.Load()
		if count < 2 {
			t.Errorf("expected at least 2 background checks, got %d", count)
		}
	})

	t.Run("get last results", func(t *testing.T) {
		c := NewChecker()

		c.RegisterFunc("test", func(ctx context.Context) error {
			return nil
		}, true)

		c.CheckAll(context.Background())
		results := c.GetLastResults()

		if len(results) != 1 {
			t.Errorf("expected 1 result, got %d", len(results))
		}
		if results["test"].Status != StatusHealthy {
			t.Errorf("expected healthy, got %s", results["test"].Status)
		}
	})
}

func TestHTTPHandlers(t *testing.T) {
	t.Run("health handler healthy", func(t *testing.T) {
		c := NewChecker()
		c.RegisterFunc("test", func(ctx context.Context) error {
			return nil
		}, true)

		req := httptest.NewRequest(http.MethodGet, "/health", nil)
		rec := httptest.NewRecorder()

		c.Handler().ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", rec.Code)
		}

		var response HealthResponse
		if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if response.Status != StatusHealthy {
			t.Errorf("expected healthy status, got %s", response.Status)
		}
	})

	t.Run("health handler unhealthy", func(t *testing.T) {
		c := NewChecker()
		c.RegisterFunc("test", func(ctx context.Context) error {
			return errors.New("error")
		}, true)

		req := httptest.NewRequest(http.MethodGet, "/health", nil)
		rec := httptest.NewRecorder()

		c.Handler().ServeHTTP(rec, req)

		if rec.Code != http.StatusServiceUnavailable {
			t.Errorf("expected status 503, got %d", rec.Code)
		}
	})

	t.Run("liveness handler always healthy", func(t *testing.T) {
		c := NewChecker()
		c.RegisterFunc("failing", func(ctx context.Context) error {
			return errors.New("error")
		}, true)

		req := httptest.NewRequest(http.MethodGet, "/live", nil)
		rec := httptest.NewRecorder()

		c.LivenessHandler().ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", rec.Code)
		}
	})

	t.Run("readiness handler checks all", func(t *testing.T) {
		c := NewChecker()
		c.RegisterFunc("test", func(ctx context.Context) error {
			return nil
		}, true)

		req := httptest.NewRequest(http.MethodGet, "/ready", nil)
		rec := httptest.NewRecorder()

		c.ReadinessHandler().ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", rec.Code)
		}
	})

	t.Run("readiness handler degraded still OK", func(t *testing.T) {
		c := NewChecker()
		c.RegisterFunc("non-critical", func(ctx context.Context) error {
			return errors.New("warning")
		}, false)

		req := httptest.NewRequest(http.MethodGet, "/ready", nil)
		rec := httptest.NewRecorder()

		c.ReadinessHandler().ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected status 200 for degraded, got %d", rec.Code)
		}
	})
}

func TestCommonChecks(t *testing.T) {
	t.Run("ping check", func(t *testing.T) {
		check := PingCheck()
		err := check(context.Background())
		if err != nil {
			t.Errorf("ping check should always succeed, got %v", err)
		}
	})

	t.Run("threshold check pass", func(t *testing.T) {
		check := ThresholdCheck("memory", func() float64 {
			return 50.0
		}, 80.0)

		err := check(context.Background())
		if err != nil {
			t.Errorf("threshold check should pass, got %v", err)
		}
	})

	t.Run("threshold check fail", func(t *testing.T) {
		check := ThresholdCheck("memory", func() float64 {
			return 90.0
		}, 80.0)

		err := check(context.Background())
		if err == nil {
			t.Error("threshold check should fail")
		}

		var thresholdErr *ThresholdError
		if !errors.As(err, &thresholdErr) {
			t.Error("expected ThresholdError")
		}
	})

	t.Run("circuit breaker check healthy", func(t *testing.T) {
		check := CircuitBreakerCheck("api", func() string {
			return "closed"
		})

		err := check(context.Background())
		if err != nil {
			t.Errorf("circuit breaker check should pass when closed, got %v", err)
		}
	})

	t.Run("circuit breaker check open", func(t *testing.T) {
		check := CircuitBreakerCheck("api", func() string {
			return "open"
		})

		err := check(context.Background())
		if err == nil {
			t.Error("circuit breaker check should fail when open")
		}

		var cbErr *CircuitOpenError
		if !errors.As(err, &cbErr) {
			t.Error("expected CircuitOpenError")
		}
	})

	t.Run("http check success", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		check := HTTPCheck(server.URL, 5*time.Second)
		err := check(context.Background())

		if err != nil {
			t.Errorf("HTTP check should pass, got %v", err)
		}
	})

	t.Run("http check failure", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer server.Close()

		check := HTTPCheck(server.URL, 5*time.Second)
		err := check(context.Background())

		if err == nil {
			t.Error("HTTP check should fail for 500 status")
		}

		var httpErr *HTTPError
		if !errors.As(err, &httpErr) {
			t.Error("expected HTTPError")
		}
	})
}

func TestStatus(t *testing.T) {
	tests := []struct {
		status   Status
		expected string
	}{
		{StatusHealthy, "healthy"},
		{StatusUnhealthy, "unhealthy"},
		{StatusDegraded, "degraded"},
		{StatusUnknown, "unknown"},
	}

	for _, tc := range tests {
		if string(tc.status) != tc.expected {
			t.Errorf("expected %s, got %s", tc.expected, tc.status)
		}
	}
}
