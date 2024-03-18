package scheduler

import (
	"encoding/json"
	"math"
	"net/http"
	"strconv"
	"sync"

	"github.com/kvaster/topols"
	corev1 "k8s.io/api/core/v1"
)

func scoreNodes(pod *corev1.Pod, nodes []corev1.Node, weights map[string]float64) []HostPriority {
	requested := extractRequestedSize(pod)
	if len(requested) == 0 {
		return nil
	}

	result := make([]HostPriority, len(nodes))
	wg := &sync.WaitGroup{}
	wg.Add(len(nodes))
	for i := range nodes {
		r := &result[i]
		item := nodes[i]
		go func() {
			score := scoreNode(item, requested, weights)
			*r = HostPriority{Host: item.Name, Score: score}
			wg.Done()
		}()
	}
	wg.Wait()

	return result
}

func scoreNode(item corev1.Node, requested map[string]int64, weights map[string]float64) int {
	totalWeight := float64(0)
	score := float64(0)

	for dc, r := range requested {
		if dc == topols.DefaultDeviceClassAnnotationName {
			var ok bool
			if dc, ok = item.Annotations[topols.DefaultDeviceClassKey]; !ok {
				// no default device class while requested - should not happen after filtering nodes
				return 0
			}
		}

		if val, ok := item.Annotations[topols.CapacityKeyPrefix+dc]; ok {
			capacity, _ := strconv.ParseInt(val, 10, 64)
			if capacity < r {
				// requested capacity is bigger when available - should not happen after filtering nodes
				return 0
			}

			var weight float64
			if weight, ok = weights[dc]; !ok {
				weight = 1
			}

			totalWeight += weight
			score += (1 - (float64(r) / float64(capacity))) * weight
		} else {
			// no requested device class found - should not happen after filtering nodes
			return 0
		}
	}

	return int(math.Round(score * 10 / totalWeight))
}

func (s scheduler) prioritize(w http.ResponseWriter, r *http.Request) {
	var input ExtenderArgs

	reader := http.MaxBytesReader(w, r.Body, 10<<20)
	err := json.NewDecoder(reader).Decode(&input)
	if err != nil {
		http.Error(w, "Bad Request.", http.StatusBadRequest)
		return
	}

	result := scoreNodes(input.Pod, input.Nodes.Items, s.weights)

	w.Header().Set("content-type", "application/json")
	_ = json.NewEncoder(w).Encode(result)
}
