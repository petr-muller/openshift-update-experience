package nodes

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	mcfgv1 "github.com/openshift/api/machineconfiguration/v1"
	"github.com/petr-muller/openshift-update-experience/internal/mco"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_MachineConfigVersionCache_Ingest(t *testing.T) {
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
			cache := &machineConfigVersionCache{}
			for _, mc := range tc.initial {
				cache.ingest(&mc)
			}
			changed, logMsg := cache.ingest(&mcfgv1.MachineConfig{ObjectMeta: tc.mc})
			if diff := cmp.Diff(tc.expectedChanged, changed); diff != "" {
				t.Errorf("unexpected changed (-want +got):\n%s", diff)
			}
			if diff := cmp.Diff(tc.expectedLogMsg, logMsg); diff != "" {
				t.Errorf("unexpected logMsg (-want +got):\n%s", diff)
			}
		})
	}
}

func Test_MachineConfigVersionCache_Forget(t *testing.T) {
	c := &machineConfigVersionCache{}
	c.ingest(&mcfgv1.MachineConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "mc1",
			Annotations: map[string]string{mco.ReleaseImageVersionAnnotationKey: "4.12.0"},
		},
	})

	changed := c.forget("mc1")
	if !changed {
		t.Errorf("expected forget to return true for existing entry")
	}
	changed = c.forget("mc1")
	if changed {
		t.Errorf("expected forget to return false for non-existing entry")
	}
}

func Test_MachineConfigVersionCache_VersionFor(t *testing.T) {
	c := &machineConfigVersionCache{}
	c.ingest(&mcfgv1.MachineConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "mc1",
			Annotations: map[string]string{mco.ReleaseImageVersionAnnotationKey: "4.12.0"},
		},
	})

	version, found := c.versionFor("mc1")
	if !found {
		t.Errorf("expected to find version for existing entry")
	}
	if version != "4.12.0" {
		t.Errorf("expected version to be '4.12.0', got '%s'", version)
	}

	version, found = c.versionFor("mc2")
	if found {
		t.Errorf("expected not to find version for non-existing entry")
	}
	if version != "" {
		t.Errorf("expected version to be '', got '%s'", version)
	}

}
