package scheduler

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/kvaster/topols"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestScoreNodes(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				topols.CapacityKeyPrefix + "dc1": fmt.Sprintf("%d", 64<<30),
				topols.CapacityKeyPrefix + "dc2": fmt.Sprintf("%d", 64<<30),
				topols.CapacityKeyPrefix + "dc3": fmt.Sprintf("%d", 64<<30),
			},
		},
	}
	input := []corev1.Node{
		testNode("10.1.1.1", 128, 128, 128),
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "10.1.1.2",
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "10.1.1.3",
				Annotations: map[string]string{
					topols.CapacityKeyPrefix + "dc1": "foo",
				},
			},
		},
	}
	expected := []HostPriority{
		{
			Host:  "10.1.1.1",
			Score: 5,
		},
		{
			Host:  "10.1.1.2",
			Score: 0,
		},
		{
			Host:  "10.1.1.3",
			Score: 0,
		},
	}

	weights := map[string]float64{
		"dc1": 1,
		"dc2": 1.5,
	}
	result := scoreNodes(pod, input, weights)
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("expected scoreNodes() to be %#v, but actual %#v", expected, result)
	}
}
