package libimage

import (
	"fmt"
	"strings"

	"github.com/disiqueira/gotree/v3"
	"github.com/docker/go-units"
)

// Tree generates a tree for the specified image and its layers.  Use
// `traverseChildren` to traverse the layers of all children.  By default, only
// layers of the image are printed.
func (i *Image) Tree(traverseChildren bool) (string, error) {
	// NOTE: a string builder prevents us from copying to much data around
	// and compile the string when and where needed.
	sb := &strings.Builder{}

	// First print the pretty header for the target image.
	size, err := i.Size()
	if err != nil {
		return "", err
	}
	repoTags, err := i.RepoTags()
	if err != nil {
		return "", err
	}

	fmt.Fprintf(sb, "Image ID: %s\n", i.ID()[:12])
	fmt.Fprintf(sb, "Tags:     %s\n", repoTags)
	fmt.Fprintf(sb, "Size:     %v\n", units.HumanSizeWithPrecision(float64(size), 4))
	if i.TopLayer() != "" {
		fmt.Fprintf(sb, "Image Layers")
	} else {
		fmt.Fprintf(sb, "No Image Layers")
	}

	tree := gotree.New(sb.String())

	layerTree, err := i.runtime.layerTree()
	if err != nil {
		return "", err
	}

	imageNode := layerTree.node(i.TopLayer())

	// Traverse the entire tree down to all children.
	if traverseChildren {
		if err := imageTreeTraverseChildren(imageNode, tree); err != nil {
			return "", err
		}
	} else {
		// Walk all layers of the image and assemlbe their data.
		for parentNode := imageNode; parentNode != nil; parentNode = parentNode.parent {
			if parentNode.layer == nil {
				break // we're done
			}
			var tags string
			repoTags, err := parentNode.repoTags()
			if err != nil {
				return "", err
			}
			if len(repoTags) > 0 {
				tags = fmt.Sprintf(" Top Layer of: %s", repoTags)
			}
			tree.Add(fmt.Sprintf("ID: %s Size: %7v%s", parentNode.layer.ID[:12], units.HumanSizeWithPrecision(float64(parentNode.layer.UncompressedSize), 4), tags))
		}
	}

	return tree.Print(), nil
}

func imageTreeTraverseChildren(node *layerNode, parent gotree.Tree) error {
	var tags string
	repoTags, err := node.repoTags()
	if err != nil {
		return err
	}
	if len(repoTags) > 0 {
		tags = fmt.Sprintf(" Top Layer of: %s", repoTags)
	}

	newNode := parent.Add(fmt.Sprintf("ID: %s Size: %7v%s", node.layer.ID[:12], units.HumanSizeWithPrecision(float64(node.layer.UncompressedSize), 4), tags))

	if len(node.children) <= 1 {
		newNode = parent
	}
	for i := range node.children {
		child := node.children[i]
		if err := imageTreeTraverseChildren(child, newNode); err != nil {
			return err
		}
	}

	return nil
}
