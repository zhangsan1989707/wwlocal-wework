package service

import (
	"context"
	"time"

	"github.com/sony/gobreaker"
	"wwlocal-wework/pkg/metrics"
)

type CircuitBreaker struct {
	cb *gobreaker.CircuitBreaker
}

func NewCircuitBreaker(name string) *CircuitBreaker {
	settings := gobreaker.Settings{
		Name: name,
		MaxRequests: 3,
		Interval: 10 * time.Second,
		Timeout: 30 * time.Second,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			failureRatio := float64(counts.TotalFailures) / float64(counts.Requests)
			return counts.ConsecutiveFailures >= 5 || failureRatio >= 0.6
		},
		OnStateChange: func(name string, from gobreaker.State, to gobreaker.State) {
			metrics.SyncOperationsTotal.WithLabelValues(name, "circuit_"+string(to)).Inc()
		},
	}

	return &CircuitBreaker{
		cb: gobreaker.NewCircuitBreaker(settings),
	}
}

func (cb *CircuitBreaker) Execute(ctx context.Context, fn func() (interface{}, error)) (interface{}, error) {
	result, err := cb.cb.Execute(func() (interface{}, error) {
		return fn()
	})

	if err != nil {
		if err == gobreaker.ErrOpenState || err == gobreaker.ErrTooManyRequests {
			return nil, &CircuitOpenError{Name: cb.cb.Name, Err: err}
		}
		return nil, err
	}

	return result, nil
}

func (cb *CircuitBreaker) State() gobreaker.State {
	return cb.cb.State()
}

func (cb *CircuitBreaker) Name() string {
	return cb.cb.Name()
}

type CircuitOpenError struct {
	Name string
	Err  error
}

func (e *CircuitOpenError) Error() string {
	return "circuit breaker '" + e.Name + "' is open: " + e.Err.Error()
}

func (e *CircuitOpenError) Unwrap() error {
	return e.Err
}
