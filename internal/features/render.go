package features

import (
	"fmt"
	"strings"
)

func RenderInstallationOrder[T id](lst []*Node[T]) string {
	sb := strings.Builder{}
	i := len(lst)
	for _, n := range lst {
		sb.WriteString(fmt.Sprintf("%s [%s]\n", n.Data.Name(), n.ID()))
		i--
		if i > 0 {
			sb.WriteString("|\n")
			sb.WriteString("∇\n")
			sb.WriteString("∇\n")
		}
	}
	return sb.String()
}

func RenderAsciiGraph[T id](root *Node[T]) string {
	if root == nil {
		return ""
	}

	var sb strings.Builder
	visited := make(map[string]struct{}) // To track visited node IDs

	// Print the root node
	sb.WriteString(fmt.Sprintf("[%s]\n", root.Data.Name()))
	visited[root.Data.ID()] = struct{}{}

	// Recursively print children of the root
	numChildren := len(root.Leafs)
	for i, leaf := range root.Leafs {
		isLastChild := i == numChildren-1
		connector := "├── "
		childContinuationPrefix := "│   " // Prefix for lines under this child's branch
		if isLastChild {
			connector = "└── "
			childContinuationPrefix = "    " // No vertical line if it's the last child
		}
		renderRecursiveInternal(&sb, leaf, connector, childContinuationPrefix, visited)
	}

	return sb.String()
}

func renderRecursiveInternal[T id](
	sb *strings.Builder,
	node *Node[T],
	currentLinePrefix string,
	nextLevelIndentPrefix string,
	visited map[string]struct{},
) {
	if node == nil {
		return
	}

	nodeID := node.Data.ID()
	nodeName := node.Data.Name()

	sb.WriteString(currentLinePrefix)
	sb.WriteString(fmt.Sprintf("[%s]", nodeName))

	if _, ok := visited[nodeID]; ok {
		sb.WriteString(" (ref...)\n") // Node already visited, indicate reference
		return
	}
	sb.WriteString("\n")
	visited[nodeID] = struct{}{}

	numChildren := len(node.Leafs)
	for i, leaf := range node.Leafs {
		isLastChild := i == numChildren-1
		connector := "├── "
		childContinuationPrefix := "│   "
		if isLastChild {
			connector = "└── "
			childContinuationPrefix = "    "
		}
		childLinePrefix := nextLevelIndentPrefix + connector
		grandChildrenIndentPrefix := nextLevelIndentPrefix + childContinuationPrefix

		renderRecursiveInternal(sb, leaf, childLinePrefix, grandChildrenIndentPrefix, visited)
	}
}
