package metrics

import (
	"sync"
	"testing"
)

func TestInMemoryMetrics(t *testing.T) {
	t.Run("Counter", func(t *testing.T) {
		m := NewInMemoryMetrics()
		counter := m.Counter("test_counter", Labels{"env": "test"})

		counter.Inc()
		counter.Inc()
		counter.Add(3)

		value := m.GetCounterValue("test_counter", Labels{"env": "test"})
		if value != 5 {
			t.Errorf("expected counter value 5, got %f", value)
		}
	})

	t.Run("Gauge", func(t *testing.T) {
		m := NewInMemoryMetrics()
		gauge := m.Gauge("test_gauge", Labels{"env": "test"})

		gauge.Set(10)
		value := m.GetGaugeValue("test_gauge", Labels{"env": "test"})
		if value != 10 {
			t.Errorf("expected gauge value 10, got %f", value)
		}

		gauge.Inc()
		value = m.GetGaugeValue("test_gauge", Labels{"env": "test"})
		if value != 11 {
			t.Errorf("expected gauge value 11, got %f", value)
		}

		gauge.Dec()
		value = m.GetGaugeValue("test_gauge", Labels{"env": "test"})
		if value != 10 {
			t.Errorf("expected gauge value 10, got %f", value)
		}

		gauge.Add(5)
		value = m.GetGaugeValue("test_gauge", Labels{"env": "test"})
		if value != 15 {
			t.Errorf("expected gauge value 15, got %f", value)
		}
	})

	t.Run("Histogram", func(t *testing.T) {
		m := NewInMemoryMetrics()
		histogram := m.Histogram("test_histogram", Labels{"env": "test"})

		histogram.Observe(1.0)
		histogram.Observe(2.0)
		histogram.Observe(3.0)

		count := m.GetHistogramCount("test_histogram", Labels{"env": "test"})
		if count != 3 {
			t.Errorf("expected histogram count 3, got %d", count)
		}
	})

	t.Run("Timer", func(t *testing.T) {
		m := NewInMemoryMetrics()
		histogram := m.Histogram("duration", nil)

		timer := m.NewTimer(histogram)
		// Do something
		timer.ObserveDuration()

		count := m.GetHistogramCount("duration", nil)
		if count != 1 {
			t.Errorf("expected timer to record 1 observation")
		}
	})

	t.Run("Reuse same metric", func(t *testing.T) {
		m := NewInMemoryMetrics()
		counter1 := m.Counter("same", nil)
		counter2 := m.Counter("same", nil)

		counter1.Inc()
		counter2.Inc()

		value := m.GetCounterValue("same", nil)
		if value != 2 {
			t.Errorf("expected same counter to be reused, got %f", value)
		}
	})
}

func TestNopMetrics(t *testing.T) {
	m := NewNopMetrics()

	// Should not panic
	counter := m.Counter("test", nil)
	counter.Inc()
	counter.Add(10)

	gauge := m.Gauge("test", nil)
	gauge.Set(10)
	gauge.Inc()
	gauge.Dec()
	gauge.Add(5)

	histogram := m.Histogram("test", nil)
	histogram.Observe(1.0)

	timer := m.NewTimer(histogram)
	timer.ObserveDuration()
}

func TestGlobalMetrics(t *testing.T) {
	m := NewInMemoryMetrics()
	SetMetrics(m)

	counter := NewCounter("global_test", nil)
	counter.Inc()

	value := m.GetCounterValue("global_test", nil)
	if value != 1 {
		t.Errorf("expected global counter to work, got %f", value)
	}

	// Reset to default
	SetMetrics(NewNopMetrics())
}

func TestConcurrentMetrics(t *testing.T) {
	m := NewInMemoryMetrics()
	counter := m.Counter("concurrent", nil)

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			counter.Inc()
		}()
	}
	wg.Wait()

	value := m.GetCounterValue("concurrent", nil)
	if value != 100 {
		t.Errorf("expected counter value 100, got %f", value)
	}
}

func TestInMemoryHistogramDetails(t *testing.T) {
	m := NewInMemoryMetrics()
	histogram := m.Histogram("test", nil).(*InMemoryHistogram)

	histogram.Observe(1.0)
	histogram.Observe(2.0)
	histogram.Observe(3.0)

	if histogram.Count() != 3 {
		t.Errorf("expected count 3, got %d", histogram.Count())
	}

	if histogram.Sum() != 6.0 {
		t.Errorf("expected sum 6.0, got %f", histogram.Sum())
	}

	values := histogram.Values()
	if len(values) != 3 {
		t.Errorf("expected 3 values, got %d", len(values))
	}
}
