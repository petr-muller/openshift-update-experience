package nodes

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels2 "k8s.io/apimachinery/pkg/labels"
)

func Test_Ingest(t *testing.T) {
	workerSelector := metav1.LabelSelector{
		MatchLabels: map[string]string{"machineconfiguration.openshift.io/role": "worker"},
	}
	masterSelector := metav1.LabelSelector{
		MatchLabels: map[string]string{"machineconfiguration.openshift.io/role": "master"},
	}

	type pool struct {
		name     string
		selector *metav1.LabelSelector
	}

	testCases := []struct {
		name    string
		initial []pool

		input pool

		expectChanged bool
		expectReason  string
	}{
		{
			name: "new pool to the empty cache",
			input: pool{
				name:     "worker",
				selector: &workerSelector,
			},
			expectChanged: true,
			expectReason:  "selector for MachineConfigPool worker changed from \"\" to \"machineconfiguration.openshift.io/role=worker\"",
		},
		{
			name: "known pool with unchanged selector",
			initial: []pool{
				{
					name:     "worker",
					selector: &workerSelector,
				},
			},
			input: pool{
				name:     "worker",
				selector: &workerSelector,
			},
			expectChanged: false,
			expectReason:  "",
		},
		{
			name: "known pool with changed selector",
			initial: []pool{
				{
					name:     "worker",
					selector: &workerSelector,
				},
			},
			input: pool{
				name:     "worker",
				selector: &masterSelector,
			},
			expectChanged: true,
			expectReason:  "selector for MachineConfigPool worker changed from \"machineconfiguration.openshift.io/role=worker\" to \"machineconfiguration.openshift.io/role=master\"",
		},
		{
			name: "new pool with invalid selector",
			input: pool{
				name:     "worker",
				selector: &metav1.LabelSelector{MatchExpressions: []metav1.LabelSelectorRequirement{{Key: "a", Operator: metav1.LabelSelectorOperator("invalid")}}},
			},
			expectChanged: false,
			expectReason:  "",
		},
		{
			name: "known pool changed to invalid selector",
			initial: []pool{
				{
					name:     "worker",
					selector: &workerSelector,
				},
			},
			input: pool{
				name:     "worker",
				selector: &metav1.LabelSelector{MatchExpressions: []metav1.LabelSelectorRequirement{{Key: "a", Operator: metav1.LabelSelectorOperator("invalid")}}},
			},
			expectChanged: true,
			expectReason:  "the previous selector \"machineconfiguration.openshift.io/role=worker\" for MachineConfigPool \"worker\" deleted as its current node selector cannot be converted to a label selector: \"invalid\" is not a valid label selector operator",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			c := machineConfigPoolSelectorCache{}
			for _, p := range tc.initial {
				if added, _ := c.ingest(p.name, p.selector); !added {
					t.Fatalf("failed to add initial pool %s", p.name)
				}
			}

			changed, reason := c.ingest(tc.input.name, tc.input.selector)
			if diff := cmp.Diff(tc.expectChanged, changed); diff != "" {
				t.Errorf("changed (-want +got):\n%s", diff)
			}

			if diff := cmp.Diff(tc.expectReason, reason); diff != "" {
				t.Errorf("reason (-want +got):\n%s", diff)
			}
		})
	}
}

func Test_Forget(t *testing.T) {
	c := machineConfigPoolSelectorCache{}
	workerSelector := metav1.LabelSelector{
		MatchLabels: map[string]string{"machineconfiguration.openshift.io/role": "worker"},
	}

	changed := c.forget("worker")
	if diff := cmp.Diff(false, changed); diff != "" {
		t.Errorf("changed (-want +got):\n%s", diff)
	}

	c.ingest("worker", &workerSelector)
	changed = c.forget("worker")
	if diff := cmp.Diff(true, changed); diff != "" {
		t.Errorf("changed (-want +got):\n%s", diff)
	}

	changed = c.forget("worker")
	if diff := cmp.Diff(false, changed); diff != "" {
		t.Errorf("changed (-want +got):\n%s", diff)
	}
}

func Test_WhichMCP(t *testing.T) {
	c := machineConfigPoolSelectorCache{}
	c.ingest("worker", &metav1.LabelSelector{
		MatchLabels: map[string]string{"machineconfiguration.openshift.io/role": "worker"},
	})
	c.ingest("master", &metav1.LabelSelector{
		MatchLabels: map[string]string{"machineconfiguration.openshift.io/role": "master"},
	})
	c.ingest("custom", &metav1.LabelSelector{
		MatchLabels: map[string]string{"machineconfiguration.openshift.io/role": "custom"},
	})

	testCases := []struct {
		name          string
		selectorLabel string
		expect        string
	}{
		{
			name:          "master node",
			selectorLabel: "machineconfiguration.openshift.io/role=master",
			expect:        "master",
		},
		{
			name:          "worker node",
			selectorLabel: "machineconfiguration.openshift.io/role=worker",
			expect:        "worker",
		},
		{
			name:          "custom node",
			selectorLabel: "machineconfiguration.openshift.io/role=custom",
			expect:        "custom",
		},
		{
			name:          "unknown node",
			selectorLabel: "machineconfiguration.openshift.io/role=unknown",
			expect:        "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			labels, err := labels2.ConvertSelectorToLabelsMap(tc.selectorLabel)
			if err != nil {
				t.Fatalf("failed to convert selector %q to labels: %v", tc.selectorLabel, err)
			}

			mcp := c.whichMCP(labels)
			if diff := cmp.Diff(tc.expect, mcp); diff != "" {
				t.Errorf("mcp (-want +got):\n%s", diff)
			}
		})
	}
}
