package nodestate

import (
	"fmt"
	"sync"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	mcfgv1 "github.com/openshift/api/machineconfiguration/v1"

	"github.com/petr-muller/openshift-update-experience/internal/mco"
)

// MachineConfigPoolSelectorCache caches label selectors from MachineConfigPools
// to efficiently determine which pool a node belongs to.
type MachineConfigPoolSelectorCache struct {
	cache sync.Map
}

// WhichMCP returns the name of the MachineConfigPool that matches the given node labels.
// Returns empty string if no match is found.
// Master pool takes precedence if a node matches multiple pools.
func (c *MachineConfigPoolSelectorCache) WhichMCP(l labels.Labels) string {
	var ret string
	c.cache.Range(func(k, v interface{}) bool {
		s := v.(labels.Selector)
		if k == mco.MachineConfigPoolMaster && s.Matches(l) {
			ret = mco.MachineConfigPoolMaster
			return false
		}
		if s.Matches(l) {
			ret = k.(string)
			return ret == mco.MachineConfigPoolWorker
		}
		return true
	})
	return ret
}

// Ingest adds or updates a cached selector for an MCP.
// Returns (modified, reason) indicating if cache changed and why.
func (c *MachineConfigPoolSelectorCache) Ingest(mcpName string, selector *metav1.LabelSelector) (bool, string) {
	s, err := metav1.LabelSelectorAsSelector(selector)
	if err != nil {
		logger := logf.Log
		logger.WithValues("MachineConfigPool", mcpName).Error(err, "Failed to convert node selector to label selector")
		v, loaded := c.cache.LoadAndDelete(mcpName)
		if loaded {
			return true, fmt.Sprintf("the previous selector %q for MachineConfigPool %q deleted as its current node selector cannot be converted to a label selector: %v", v, mcpName, err)
		}
		return false, ""
	}

	previous, loaded := c.cache.Swap(mcpName, s)
	if !loaded || previous.(labels.Selector).String() != s.String() {
		var vStr string
		if loaded {
			vStr = previous.(labels.Selector).String()
		}
		return true, fmt.Sprintf("selector for MachineConfigPool %s changed from %q to %q", mcpName, vStr, s.String())
	}
	return false, ""
}

// Forget removes a cached selector.
// Returns true if the selector was present and removed.
func (c *MachineConfigPoolSelectorCache) Forget(mcpName string) bool {
	_, loaded := c.cache.LoadAndDelete(mcpName)
	return loaded
}

// MachineConfigVersionCache caches the OCP version extracted from MachineConfig
// annotations for efficient version lookup during node state evaluation.
type MachineConfigVersionCache struct {
	cache sync.Map
}

// Ingest adds or updates a version mapping from a MachineConfig resource.
// Returns (modified, reason) indicating if cache changed and why.
func (c *MachineConfigVersionCache) Ingest(mc *mcfgv1.MachineConfig) (bool, string) {
	if mcVersion, annotated := mc.Annotations[mco.ReleaseImageVersionAnnotationKey]; annotated && mcVersion != "" {
		previous, loaded := c.cache.Swap(mc.Name, mcVersion)
		if !loaded || previous.(string) != mcVersion {
			var previousStr string
			if loaded {
				previousStr = previous.(string)
			}
			return true, fmt.Sprintf("version for MachineConfig %s changed from %s to %s", mc.Name, previousStr, mcVersion)
		}
		return false, ""
	}

	_, loaded := c.cache.LoadAndDelete(mc.Name)
	if loaded {
		return true, fmt.Sprintf("the previous version for MachineConfig %s deleted as no version can be found now", mc.Name)
	}
	return false, ""
}

// Forget removes a cached version mapping.
// Returns true if the mapping was present and removed.
func (c *MachineConfigVersionCache) Forget(name string) bool {
	_, loaded := c.cache.LoadAndDelete(name)
	return loaded
}

// VersionFor returns the OCP version for a MachineConfig.
// Returns (version, true) if found, ("", false) otherwise.
func (c *MachineConfigVersionCache) VersionFor(key string) (string, bool) {
	v, loaded := c.cache.Load(key)
	if !loaded {
		return "", false
	}
	return v.(string), true
}
