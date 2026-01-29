package nodestate

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	mcfgv1 "github.com/openshift/api/machineconfiguration/v1"
	"github.com/petr-muller/openshift-update-experience/internal/mco"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels2 "k8s.io/apimachinery/pkg/labels"
)

func TestMachineConfigPoolSelectorCache_Ingest(t *testing.T) {
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
			c := MachineConfigPoolSelectorCache{}
			for _, p := range tc.initial {
				if added, _ := c.Ingest(p.name, p.selector); !added {
					t.Fatalf("failed to add initial pool %s", p.name)
				}
			}

			changed, reason := c.Ingest(tc.input.name, tc.input.selector)
			if diff := cmp.Diff(tc.expectChanged, changed); diff != "" {
				t.Errorf("changed (-want +got):\n%s", diff)
			}

			if diff := cmp.Diff(tc.expectReason, reason); diff != "" {
				t.Errorf("reason (-want +got):\n%s", diff)
			}
		})
	}
}

func TestMachineConfigPoolSelectorCache_Forget(t *testing.T) {
	c := MachineConfigPoolSelectorCache{}
	workerSelector := metav1.LabelSelector{
		MatchLabels: map[string]string{"machineconfiguration.openshift.io/role": "worker"},
	}

	changed := c.Forget("worker")
	if diff := cmp.Diff(false, changed); diff != "" {
		t.Errorf("changed (-want +got):\n%s", diff)
	}

	c.Ingest("worker", &workerSelector)
	changed = c.Forget("worker")
	if diff := cmp.Diff(true, changed); diff != "" {
		t.Errorf("changed (-want +got):\n%s", diff)
	}

	changed = c.Forget("worker")
	if diff := cmp.Diff(false, changed); diff != "" {
		t.Errorf("changed (-want +got):\n%s", diff)
	}
}

func TestMachineConfigPoolSelectorCache_WhichMCP(t *testing.T) {
	c := MachineConfigPoolSelectorCache{}
	c.Ingest("worker", &metav1.LabelSelector{
		MatchLabels: map[string]string{"machineconfiguration.openshift.io/role": "worker"},
	})
	c.Ingest("master", &metav1.LabelSelector{
		MatchLabels: map[string]string{"machineconfiguration.openshift.io/role": "master"},
	})
	c.Ingest("custom", &metav1.LabelSelector{
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

			mcp := c.WhichMCP(labels)
			if diff := cmp.Diff(tc.expect, mcp); diff != "" {
				t.Errorf("mcp (-want +got):\n%s", diff)
			}
		})
	}
}

func TestMachineConfigVersionCache_Ingest(t *testing.T) {
	testCases := []struct {
		name            string
		initial         []mcfgv1.MachineConfig
		mc              metav1.ObjectMeta
		expectedChanged bool
		expectedLogMsg  string
	}{
		{
			name: "new MC with version",
			mc: metav1.ObjectMeta{
				Name: "mc1",
				Annotations: map[string]string{
					mco.ReleaseImageVersionAnnotationKey: "4.12.0",
				},
			},
			expectedChanged: true,
			expectedLogMsg:  "version for MachineConfig mc1 changed from  to 4.12.0",
		},
		{
			name: "new MC without version",
			mc: metav1.ObjectMeta{
				Name: "mc1",
			},
			expectedChanged: false,
			expectedLogMsg:  "",
		},
		{
			name: "existing MC with same version",
			initial: []mcfgv1.MachineConfig{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:        "mc1",
						Annotations: map[string]string{mco.ReleaseImageVersionAnnotationKey: "4.12.0"},
					},
				},
			},
			mc: metav1.ObjectMeta{
				Name:        "mc1",
				Annotations: map[string]string{mco.ReleaseImageVersionAnnotationKey: "4.12.0"},
			},
			expectedChanged: false,
			expectedLogMsg:  "",
		},
		{
			name: "existing MC with different version",
			initial: []mcfgv1.MachineConfig{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:        "mc1",
						Annotations: map[string]string{mco.ReleaseImageVersionAnnotationKey: "4.12.0"},
					},
				},
			},
			mc: metav1.ObjectMeta{
				Name:        "mc1",
				Annotations: map[string]string{mco.ReleaseImageVersionAnnotationKey: "4.13.0"},
			},
			expectedChanged: true,
			expectedLogMsg:  "version for MachineConfig mc1 changed from 4.12.0 to 4.13.0",
		},
		{
			name: "existing MC with version removed",
			initial: []mcfgv1.MachineConfig{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:        "mc1",
						Annotations: map[string]string{mco.ReleaseImageVersionAnnotationKey: "4.12.0"},
					},
				},
			},
			mc: metav1.ObjectMeta{
				Name: "mc1",
			},
			expectedChanged: true,
			expectedLogMsg:  "the previous version for MachineConfig mc1 deleted as no version can be found now",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cache := &MachineConfigVersionCache{}
			for _, mc := range tc.initial {
				cache.Ingest(&mc)
			}
			changed, logMsg := cache.Ingest(&mcfgv1.MachineConfig{ObjectMeta: tc.mc})
			if diff := cmp.Diff(tc.expectedChanged, changed); diff != "" {
				t.Errorf("unexpected changed (-want +got):\n%s", diff)
			}
			if diff := cmp.Diff(tc.expectedLogMsg, logMsg); diff != "" {
				t.Errorf("unexpected logMsg (-want +got):\n%s", diff)
			}
		})
	}
}

func TestMachineConfigVersionCache_Forget(t *testing.T) {
	c := &MachineConfigVersionCache{}
	c.Ingest(&mcfgv1.MachineConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "mc1",
			Annotations: map[string]string{mco.ReleaseImageVersionAnnotationKey: "4.12.0"},
		},
	})

	changed := c.Forget("mc1")
	if !changed {
		t.Errorf("expected forget to return true for existing entry")
	}
	changed = c.Forget("mc1")
	if changed {
		t.Errorf("expected forget to return false for non-existing entry")
	}
}

func TestMachineConfigVersionCache_VersionFor(t *testing.T) {
	c := &MachineConfigVersionCache{}
	c.Ingest(&mcfgv1.MachineConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "mc1",
			Annotations: map[string]string{mco.ReleaseImageVersionAnnotationKey: "4.12.0"},
		},
	})

	version, found := c.VersionFor("mc1")
	if !found {
		t.Errorf("expected to find version for existing entry")
	}
	if version != "4.12.0" {
		t.Errorf("expected version to be '4.12.0', got '%s'", version)
	}

	version, found = c.VersionFor("mc2")
	if found {
		t.Errorf("expected not to find version for non-existing entry")
	}
	if version != "" {
		t.Errorf("expected version to be '', got '%s'", version)
	}
}
