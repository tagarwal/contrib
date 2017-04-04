package obmcs

import (
	"fmt"
	"k8s.io/contrib/cluster-autoscaler/cloudprovider"
	apiv1 "k8s.io/kubernetes/pkg/api/v1"
	"strconv"
	"strings"
	"sync"
	"github.com/golang/glog"
)

func BuildObmcsCloudProvider(obmcsManager *ObmcsManager, specs []string) (*ObmcsCloudProvider, error) {
	obmcs := &ObmcsCloudProvider{
		obmcsManager: obmcsManager,
		nodeGroups:   make([]*ObmcsNodeGroup, 0),
		nodes:		  make(map[string]string),
		groups:		  make(map[string]cloudprovider.NodeGroup),
	}
	for _, spec := range specs {
		if err := obmcs.addNodeGroup(spec); err != nil {
			return nil, err
		}
	}
	return obmcs, nil
}

type ObmcsCloudProvider struct {
	sync.Mutex
	obmcsManager *ObmcsManager
	nodeGroups   []*ObmcsNodeGroup
	nodes        map[string]string
	groups     map[string]cloudprovider.NodeGroup
	onIncrease func(string, int) error
	onDelete   func(string, string) error
}


// TODO For now just add the list of host machines later change it once we have the list of nodes for
// a given group


func (obmcscp *ObmcsCloudProvider) addNodeGroup(spec string) error {
	obmcsNode, err := buildNodeGroup(spec, obmcscp)
	if err != nil {
		return err
	}
	fmt.Sprintf("trying to build node group for %s", obmcsNode.id)
	obmcscp.nodeGroups = append(obmcscp.nodeGroups, obmcsNode)
	obmcscp.groups[obmcsNode.id] = obmcsNode

	//tmpNodeNames := [4]string{ "sca00kee", "sca00kec", "sca00jzl", "sca00kef" }
	//
	//for _, node := range tmpNodeNames {
	//	obmcscp.AddNode(obmcsNode.id,node)
	//}
	return nil
}

// AddNode adds the given node to the group.
func (obmcscp *ObmcsCloudProvider) AddNode(nodeGroupId string, nodeName string) {
	obmcscp.Lock()
	defer obmcscp.Unlock()
	obmcscp.nodes[nodeName] = nodeGroupId
}


// OnIncreaseFunc is a function called on node group increase in TestCloudProvider.
// First parameter is the NodeGroup id, second is the increase delta.
type OnIncreaseFunc func(string, int) error

// OnDeleteFunc is a function called on cluster
type OnDeleteFunc func(string, string) error

// Name returns name of the cloud provider.
func (obmcscp *ObmcsCloudProvider) Name() string {
	return "Oracle Bare Metal"
}

// NodeGroups returns all node groups configured for this cloud provider.
func (obmcscp *ObmcsCloudProvider) NodeGroups() []cloudprovider.NodeGroup {
	obmcscp.Lock()
	defer obmcscp.Unlock()

	result := make([]cloudprovider.NodeGroup, 0)
	for _, group := range obmcscp.nodeGroups {
		result = append(result, group)
	}
	return result
}

// NodeGroupForNode returns the node group for the given node, nil if the node
// should not be processed by cluster autoscaler, or non-nil error if such
// occurred.
func (obmcscp *ObmcsCloudProvider) NodeGroupForNode(node *apiv1.Node) (cloudprovider.NodeGroup, error) {
	obmcscp.Lock()
	defer obmcscp.Unlock()

	//groupName, found := obmcscp.nodes[node.Name]
	//if !found {
	//	return nil, nil
	//}
	//
	//group, found := obmcscp.groups[groupName]
	//if !found {
	//	return nil, nil
	//}
	////for now just return nil, nil as OBMCS does not have a concept of auto scaler or node groups
	////TODO once OBMCS has the concept of node groups change this code to check for it
	return  obmcscp.nodeGroups[0], nil
}

// TestNodeGroup is a node group used by TestCloudProvider.
type ObmcsNodeGroup struct {
	sync.Mutex
	cloudProvider *ObmcsCloudProvider
	id            string
	maxSize       int
	minSize       int
	targetSize    int
}

// MaxSize returns maximum size of the node group.
func (tng *ObmcsNodeGroup) MaxSize() int {
	tng.Lock()
	defer tng.Unlock()

	return tng.maxSize
}

// MinSize returns minimum size of the node group.
func (tng *ObmcsNodeGroup) MinSize() int {
	tng.Lock()
	defer tng.Unlock()

	return tng.minSize
}

// TargetSize returns the current target size of the node group. It is possible that the
// number of nodes in Kubernetes is different at the moment but should be equal
// to Size() once everything stabilizes (new nodes finish startup and registration or
// removed nodes are deleted completely)
func (obmcs_node_group *ObmcsNodeGroup) TargetSize() (int, error) {
	obmcs_node_group.Lock()
	defer obmcs_node_group.Unlock()

	return obmcs_node_group.cloudProvider.obmcsManager.GetOBMCSClusterSize()
}

// IncreaseSize increases the size of the node group. To delete a node you need
// to explicitly name it and use DeleteNode. This function should wait until
// node group size is updated.
func (tng *ObmcsNodeGroup) IncreaseSize(delta int) error {
	tng.Lock()
	tng.targetSize += delta
	tng.Unlock()

	if delta <= 0 {
		return fmt.Errorf("size increase must be positive")
	}
	size, err := tng.TargetSize()
	if err != nil {
		glog.Fatalf("Error while getting size %v", err)
	}

	if int(size)+delta > tng.MaxSize() {
		return fmt.Errorf("size increase too large - desired:%d max:%d", int(size)+delta, tng.MaxSize())
	}
	return tng.cloudProvider.obmcsManager.ScaleUpNode(tng, size+delta)
}

// DecreaseTargetSize decreases the target size of the node group. This function
// doesn't permit to delete any existing node and can be used only to reduce the
// request for new nodes that have not been yet fulfilled. Delta should be negative.
func (tng *ObmcsNodeGroup) DecreaseTargetSize(delta int) error {
	tng.Lock()
	tng.targetSize += delta
	tng.Unlock()

	return tng.cloudProvider.onIncrease(tng.id, delta)
}

// DeleteNodes deletes nodes from this node group. Error is returned either on
// failure or if the given node doesn't belong to this node group. This function
// should wait until node group size is updated.
func (tng *ObmcsNodeGroup) DeleteNodes(nodes []*apiv1.Node) error {
	tng.Lock()
	id := tng.id
	tng.targetSize -= len(nodes)
	tng.Unlock()
	for _, node := range nodes {
		err := tng.cloudProvider.obmcsManager.DeleteInstance(id, node.Name)
		if err != nil {
			return err
		}
	}
	return nil
}

// Id returns an unique identifier of the node group.
func (tng *ObmcsNodeGroup) Id() string {
	tng.Lock()
	defer tng.Unlock()

	return tng.id
}

// Debug returns a string containing all information regarding this node group.
func (tng *ObmcsNodeGroup) Debug() string {
	tng.Lock()
	defer tng.Unlock()

	return fmt.Sprintf("%s target:%d min:%d max:%d", tng.id, tng.targetSize, tng.minSize, tng.maxSize)
}

// Nodes returns a list of all nodes that belong to this node group.
func (tng *ObmcsNodeGroup) Nodes() ([]string, error) {
	tng.Lock()
	defer tng.Unlock()

	result := make([]string, 0)
	for node, nodegroup := range tng.cloudProvider.nodes {
		if nodegroup == tng.id {
			result = append(result, node)
		}
	}
	return result, nil
}

func buildNodeGroup(value string, cloudprovider *ObmcsCloudProvider) (*ObmcsNodeGroup, error) {
	tokens := strings.SplitN(value, ":", 3)
	if len(tokens) != 3 {
		return nil, fmt.Errorf("wrong nodes configuration: %s", value)
	}

	asg := ObmcsNodeGroup{
		cloudProvider: cloudprovider,
	}

	if size, err := strconv.Atoi(tokens[0]); err == nil {
		if size <= 0 {
			return nil, fmt.Errorf("min size must be >= 1")
		}
		asg.minSize = size
	} else {
		return nil, fmt.Errorf("failed to set min size: %s, expected integer", tokens[0])
	}

	if size, err := strconv.Atoi(tokens[1]); err == nil {
		if size < asg.minSize {
			return nil, fmt.Errorf("max size must be greater or equal to min size")
		}
		asg.maxSize = size
	} else {
		return nil, fmt.Errorf("failed to set max size: %s, expected integer", tokens[1])
	}

	if tokens[2] == "" {
		return nil, fmt.Errorf("asg name must not be blank: %s got error: %v", tokens[2])
	}

	asg.id = tokens[2]
	return &asg, nil
}
