package image

import (
	"context"

	ociv1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
)

// layerTree is an internal representation of local layers.
type layerTree struct {
	// nodes is the actual layer tree with layer IDs being keys.
	nodes map[string]*layerNode
	// ociCache is a cache for Image.ID -> OCI Image. Translations are done
	// on-demand.
	ociCache map[string]*ociv1.Image
}

// node returns a layerNode for the specified layerID.
func (t *layerTree) node(layerID string) *layerNode {
	node, exists := t.nodes[layerID]
	if !exists {
		node = &layerNode{}
		t.nodes[layerID] = node
	}
	return node
}

// toOCI returns an OCI image for the specified image.
func (t *layerTree) toOCI(ctx context.Context, i *Image) (*ociv1.Image, error) {
	var err error
	oci, exists := t.ociCache[i.ID()]
	if !exists {
		oci, err = i.ociv1Image(ctx)
		if err == nil {
			t.ociCache[i.ID()] = oci
		}
	}
	return oci, err
}

// layerNode is a node in a layerTree.  It's ID is the key in a layerTree.
type layerNode struct {
	children []*layerNode
	images   []*Image
	parent   *layerNode
}

// layerTree extracts a layerTree from the layers in the local storage and
// relates them to the specified images.
func (ir *Runtime) layerTree() (*layerTree, error) {
	layers, err := ir.store.Layers()
	if err != nil {
		return nil, err
	}

	images, err := ir.GetImages()
	if err != nil {
		return nil, err
	}

	tree := layerTree{
		nodes:    make(map[string]*layerNode),
		ociCache: make(map[string]*ociv1.Image),
	}

	// First build a tree purely based on layer information.
	for _, layer := range layers {
		node := tree.node(layer.ID)
		if layer.Parent == "" {
			continue
		}
		parent := tree.node(layer.Parent)
		node.parent = parent
		parent.children = append(parent.children, node)
	}

	// Now assign the images to each (top) layer.
	for i := range images {
		img := images[i] // do not leak loop variable outside the scope
		topLayer := img.TopLayer()
		if topLayer == "" {
			continue
		}
		node, exists := tree.nodes[topLayer]
		if !exists {
			return nil, errors.Errorf("top layer %s of image %s not found in layer tree", img.TopLayer(), img.ID())
		}
		node.images = append(node.images, img)
	}

	return &tree, nil
}

// children returns the image IDs of children . Child images are images
// with either the same top layer as parent or parent being the true parent
// layer.  Furthermore, the history of the parent and child images must match
// with the parent having one history item less.
// If all is true, all images are returned.  Otherwise, the first image is
// returned.
func (t *layerTree) children(ctx context.Context, parent *Image, all bool) ([]string, error) {
	if parent.TopLayer() == "" {
		return nil, nil
	}

	var children []string

	parentNode, exists := t.nodes[parent.TopLayer()]
	if !exists {
		return nil, errors.Errorf("layer not found in layer tree: %q", parent.TopLayer())
	}

	parentID := parent.ID()
	parentOCI, err := t.toOCI(ctx, parent)
	if err != nil {
		return nil, err
	}

	// checkParent returns true if child and parent are in such a relation.
	checkParent := func(child *Image) (bool, error) {
		if parentID == child.ID() {
			return false, nil
		}
		childOCI, err := t.toOCI(ctx, child)
		if err != nil {
			return false, err
		}
		// History check.
		return areParentAndChild(parentOCI, childOCI), nil
	}

	// addChildrenFrom adds child images of parent to children.  Returns
	// true if any image is a child of parent.
	addChildrenFromNode := func(node *layerNode) (bool, error) {
		foundChildren := false
		for _, childImage := range node.images {
			isChild, err := checkParent(childImage)
			if err != nil {
				return foundChildren, err
			}
			if isChild {
				foundChildren = true
				children = append(children, childImage.ID())
				if all {
					return foundChildren, nil
				}
			}
		}
		return foundChildren, nil
	}

	// First check images where parent's top layer is also the parent
	// layer.
	for _, childNode := range parentNode.children {
		found, err := addChildrenFromNode(childNode)
		if err != nil {
			return nil, err
		}
		if found && all {
			return children, nil
		}
	}

	// Now check images with the same top layer.
	if _, err := addChildrenFromNode(parentNode); err != nil {
		return nil, err
	}

	return children, nil
}

// parent returns the parent image or nil if no parent image could be found.
func (t *layerTree) parent(ctx context.Context, child *Image) (*Image, error) {
	if child.TopLayer() == "" {
		return nil, nil
	}

	node, exists := t.nodes[child.TopLayer()]
	if !exists {
		return nil, errors.Errorf("layer not found in layer tree: %q", child.TopLayer())
	}

	childOCI, err := t.toOCI(ctx, child)
	if err != nil {
		return nil, err
	}

	// Check images from the parent node (i.e., parent layer) and images
	// with the same layer (i.e., same top layer).
	childID := child.ID()
	images := node.images
	if node.parent != nil {
		images = append(images, node.parent.images...)
	}
	for _, parent := range images {
		if parent.ID() == childID {
			continue
		}
		parentOCI, err := t.toOCI(ctx, parent)
		if err != nil {
			return nil, err
		}
		// History check.
		if areParentAndChild(parentOCI, childOCI) {
			return parent, nil
		}
	}

	return nil, nil
}

// hasChildrenAndParent returns true if the specified image has children and a
// parent.
func (t *layerTree) hasChildrenAndParent(ctx context.Context, i *Image) (bool, error) {
	children, err := t.children(ctx, i, false)
	if err != nil {
		return false, err
	}
	if len(children) == 0 {
		return false, nil
	}
	parent, err := t.parent(ctx, i)
	return parent != nil, err
}
