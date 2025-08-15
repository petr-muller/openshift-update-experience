package nodes

import (
	"fmt"
	"sync"

	openshiftmachineconfigurationv1 "github.com/openshift/api/machineconfiguration/v1"
	"github.com/petr-muller/openshift-update-experience/internal/mco"
)

type machineConfigVersionCache struct {
	cache sync.Map
}

func (c *machineConfigVersionCache) ingest(mc *openshiftmachineconfigurationv1.MachineConfig) (bool, string) {
	if mcVersion, annotated := mc.Annotations[mco.ReleaseImageVersionAnnotationKey]; annotated && mcVersion != "" {
		previous, loaded := c.cache.Swap(mc.Name, mcVersion)
		if !loaded || previous.(string) != mcVersion {
			var previousStr string
			if loaded {
				previousStr = previous.(string)
			}
			return true, fmt.Sprintf("version for MachineConfig %s changed from %s from %s", mc.Name, previousStr, mcVersion)
		} else {
			return false, ""
		}
	}

	_, loaded := c.cache.LoadAndDelete(mc.Name)
	if loaded {
		return true, fmt.Sprintf("the previous version for MachineConfig %s deleted as no version can be found now", mc.Name)
	}
	return false, ""
}

func (c *machineConfigVersionCache) forget(name string) bool {
	_, loaded := c.cache.LoadAndDelete(name)
	return loaded
}

func (c *machineConfigVersionCache) versionFor(key string) (string, bool) {
	v, loaded := c.cache.Load(key)
	if !loaded {
		return "", false
	}
	return v.(string), true
}
