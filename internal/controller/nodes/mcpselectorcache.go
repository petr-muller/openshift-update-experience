package nodes

import (
	"fmt"
	"sync"

	openshiftmachineconfigurationv1 "github.com/openshift/api/machineconfiguration/v1"
	"github.com/petr-muller/openshift-update-experience/internal/mco"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

type machineConfigPoolSelectorCache struct {
	cache sync.Map
}

func (c *machineConfigPoolSelectorCache) whichMCP(l labels.Labels) string {
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

func (c *machineConfigPoolSelectorCache) ingest(pool *openshiftmachineconfigurationv1.MachineConfigPool) (bool, string) {
	s, err := metav1.LabelSelectorAsSelector(pool.Spec.NodeSelector)
	if err != nil {
		logger := logf.Log
		logger.WithValues("MachineConfigPool", pool.Name).Error(err, "Failed to convert node selector to label selector")
		v, loaded := c.cache.LoadAndDelete(pool.Name)
		if loaded {
			return true, fmt.Sprintf("the previous selector %s for MachineConfigPool %s deleted as its current node selector cannot be converted to a label selector: %v", v, pool.Name, err)
		} else {
			return false, ""
		}
	}

	previous, loaded := c.cache.Swap(pool.Name, s)
	if !loaded || previous.(labels.Selector).String() != s.String() {
		var vStr string
		if loaded {
			vStr = previous.(labels.Selector).String()
		}
		return true, fmt.Sprintf("selector for MachineConfigPool %s changed from %s to %s", pool.Name, vStr, s.String())
	}
	return false, ""
}

func (c *machineConfigPoolSelectorCache) forget(mcpName string) bool {
	_, loaded := c.cache.LoadAndDelete(mcpName)
	return loaded
}
