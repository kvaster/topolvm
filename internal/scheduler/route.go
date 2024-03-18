package scheduler

import (
	"fmt"
	"net/http"
)

type scheduler struct {
	weights map[string]float64
}

func (s scheduler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/predicate":
		s.predicate(w, r)
	case "/prioritize":
		s.prioritize(w, r)
	case "/status":
		status(w, r)
	default:
		http.Error(w, "not found", http.StatusNotFound)
	}
}

// NewHandler return new http.Handler of the scheduler extender
func NewHandler(weights map[string]float64) (http.Handler, error) {
	for _, weight := range weights {
		if weight <= 0 {
			return nil, fmt.Errorf("invalid weight: %f", weight)
		}
	}
	return scheduler{weights}, nil
}

func status(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}
