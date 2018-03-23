package libpod

import (
	"github.com/pkg/errors"
)

type containerNode struct {
	id         string
	container  *Container
	dependsOn  []*containerNode
	dependedOn []*containerNode
}

type containerGraph struct {
	nodes              map[string]*containerNode
	noDepNodes         []*containerNode
	notDependedOnNodes []*containerNode
}

func buildContainerGraph(ctrs []*Container) (*containerGraph, error) {
	graph := new(containerGraph)
	graph.nodes = make(map[string]*containerNode)

	// Start by building all nodes, with no edges
	for _, ctr := range ctrs {
		ctrNode := new(containerNode)
		ctrNode.id = ctr.ID()
		ctrNode.container = ctr

		graph.nodes[ctr.ID()] = ctrNode
	}

	// Now add edges based on dependencies
	for _, node := range graph.nodes {
		deps := node.container.Dependencies()
		for _, dep := range deps {
			// Get the dep's node
			depNode, ok := graph.nodes[dep]
			if !ok {
				return nil, errors.Wrapf(ErrNoSuchCtr, "container %s depends on container %s not found in input list", node.id, dep)
			}

			// Add the dependent node to the node's dependencies
			// And add the node to the dependent node's dependedOn
			node.dependsOn = append(node.dependsOn, depNode)
			depNode.dependedOn = append(depNode.dependedOn, node)
		}

		// Maintain a list of nodes with no dependencies
		// (no edges coming from them)
		if len(deps) == 0 {
			graph.noDepNodes = append(graph.noDepNodes, node)
		}
	}

	// Need one more loop to get nodes that have nothing depending on them
	// (no edges pointing to them)
	for _, node := range graph.nodes {
		if len(node.dependedOn) == 0 {
			graph.notDependedOnNodes = append(graph.notDependedOnNodes, node)
		}
	}

	// Need to do cycle detection
	// We cannot start or stop if there are cyclic dependencies
	cycle, err := detectCycles(graph)
	if err != nil {
		return nil, err
	} else if cycle {
		return nil, errors.Wrapf(ErrInternal, "cycle found in container dependency graph")
	}

	return graph, nil
}

// Detect cycles in a container graph
// Return true if a cycle is found, false otherwise
func detectCycles(graph *containerGraph) (bool, error) {
	type nodeInfo struct {
		index   int
		lowLink int
		onStack bool
	}

	index := 0

	nodes := make(map[string]*nodeInfo)
	stack := make([]*containerNode, 0, len(graph.nodes))

	var strongConnect func(*containerNode) (bool, error)
	strongConnect = func(node *containerNode) (bool, error) {
		info := new(nodeInfo)
		info.index = index
		info.lowLink = index
		index = index + 1

		nodes[node.id] = info

		stack = append(stack, node)

		info.onStack = true

		// Work through all nodes we point to
		for _, successor := range node.dependsOn {
			if _, ok := nodes[successor.id]; !ok {
				cycle, err := strongConnect(successor)
				if err != nil {
					return false, err
				} else if cycle {
					return true, nil
				}

				successorInfo := nodes[successor.id]
				if successorInfo.lowLink < info.lowLink {
					info.lowLink = successorInfo.lowLink
				}
			} else {
				successorInfo := nodes[successor.id]
				if successorInfo.index < info.lowLink && successorInfo.onStack {
					info.lowLink = successorInfo.index
				}
			}
		}

		if info.lowLink == info.index {
			l := len(stack)
			if l == 0 {
				return false, errors.Wrapf(ErrInternal, "empty stack in detectCycles")
			}

			// Pop off the stack
			topOfStack := stack[l-1]
			stack = stack[:l-1]

			// If the top of the stack is not us, we have found a
			// cycle
			if topOfStack.id != node.id {
				return true, nil
			}
		}

		return false, nil
	}

	for id, node := range graph.nodes {
		if _, ok := nodes[id]; !ok {
			cycle, err := strongConnect(node)
			if err != nil {
				return false, err
			} else if cycle {
				return true, nil
			}
		}
	}

	return false, nil
}
