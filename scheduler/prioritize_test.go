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
				topols.CapacityKeyPrefix + "ssd":  fmt.Sprintf("%d", 64<<30),
				topols.CapacityKeyPrefix + "hdd1": fmt.Sprintf("%d", 64<<30),
				topols.CapacityKeyPrefix + "hdd2": fmt.Sprintf("%d", 64<<30),
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
					topols.CapacityKeyPrefix + "ssd": "foo",
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
		"ssd":  1,
		"hdd1": 1.5,
	}
	result := scoreNodes(pod, input, weights)
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("expected scoreNodes() to be %#v, but actual %#v", expected, result)
	}
}
