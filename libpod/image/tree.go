package image

import (
	"context"
	"fmt"
	"strings"

	"github.com/docker/go-units"
	"github.com/pkg/errors"
)

const (
	middleItem   = "├── "
	continueItem = "│   "
	lastItem     = "└── "
)

type tree struct {
	img       *Image
	imageInfo *InfoImage
	layerInfo map[string]*LayerInfo
	sb        *strings.Builder
}

// GenerateTree creates an image tree string representation for displaying it
// to the user.
func (i *Image) GenerateTree(whatRequires bool) (string, error) {
	// Fetch map of image-layers, which is used for printing output.
	layerInfo, err := GetLayersMapWithImageInfo(i.imageruntime)
	if err != nil {
		return "", errors.Wrapf(err, "error while retrieving layers of image %q", i.InputName)
	}

	// Create an imageInfo and fill the image and layer info
	imageInfo := &InfoImage{
		ID:   i.ID(),
		Tags: i.Names(),
	}

	if err := BuildImageHierarchyMap(imageInfo, layerInfo, i.TopLayer()); err != nil {
		return "", err
	}
	sb := &strings.Builder{}
	tree := &tree{i, imageInfo, layerInfo, sb}
	if err := tree.print(whatRequires); err != nil {
		return "", err
	}
	return tree.string(), nil
}

func (t *tree) string() string {
	return t.sb.String()
}

func (t *tree) print(whatRequires bool) error {
	size, err := t.img.Size(context.Background())
	if err != nil {
		return err
	}

	fmt.Fprintf(t.sb, "Image ID: %s\n", t.imageInfo.ID[:12])
	fmt.Fprintf(t.sb, "Tags:     %s\n", t.imageInfo.Tags)
	fmt.Fprintf(t.sb, "Size:     %v\n", units.HumanSizeWithPrecision(float64(*size), 4))
	if t.img.TopLayer() != "" {
		fmt.Fprintf(t.sb, "Image Layers\n")
	} else {
		fmt.Fprintf(t.sb, "No Image Layers\n")
	}

	if !whatRequires {
		// fill imageInfo with layers associated with image.
		// the layers will be filled such that
		// (Start)RootLayer->...intermediate Parent Layer(s)-> TopLayer(End)
		// Build output from imageInfo into buffer
		t.printImageHierarchy(t.imageInfo)
	} else {
		// fill imageInfo with layers associated with image.
		// the layers will be filled such that
		// (Start)TopLayer->...intermediate Child Layer(s)-> Child TopLayer(End)
		//     (Forks)... intermediate Child Layer(s) -> Child Top Layer(End)
		return t.printImageChildren(t.layerInfo, t.img.TopLayer(), "", true)
	}
	return nil
}

// Stores all children layers which are created using given Image.
// Layers are stored as follows
// (Start)TopLayer->...intermediate Child Layer(s)-> Child TopLayer(End)
//             (Forks)... intermediate Child Layer(s) -> Child Top Layer(End)
func (t *tree) printImageChildren(layerMap map[string]*LayerInfo, layerID string, prefix string, last bool) error {
	if layerID == "" {
		return nil
	}
	ll, ok := layerMap[layerID]
	if !ok {
		return fmt.Errorf("lookup error: layerid  %s, not found", layerID)
	}
	fmt.Fprint(t.sb, prefix)

	//initialize intend with middleItem to reduce middleItem checks.
	intend := middleItem
	if !last {
		// add continueItem i.e. '|' for next iteration prefix
		prefix += continueItem
	} else if len(ll.ChildID) > 1 || len(ll.ChildID) == 0 {
		// The above condition ensure, alignment happens for node, which has more then 1 children.
		// If node is last in printing hierarchy, it should not be printed as middleItem i.e. ├──
		intend = lastItem
		prefix += " "
	}

	var tags string
	if len(ll.RepoTags) > 0 {
		tags = fmt.Sprintf(" Top Layer of: %s", ll.RepoTags)
	}
	fmt.Fprintf(t.sb, "%sID: %s Size: %7v%s\n", intend, ll.ID[:12], units.HumanSizeWithPrecision(float64(ll.Size), 4), tags)
	for count, childID := range ll.ChildID {
		if err := t.printImageChildren(layerMap, childID, prefix, count == len(ll.ChildID)-1); err != nil {
			return err
		}
	}
	return nil
}

// prints the layers info of image
func (t *tree) printImageHierarchy(imageInfo *InfoImage) {
	for count, l := range imageInfo.Layers {
		var tags string
		intend := middleItem
		if len(l.RepoTags) > 0 {
			tags = fmt.Sprintf(" Top Layer of: %s", l.RepoTags)
		}
		if count == len(imageInfo.Layers)-1 {
			intend = lastItem
		}
		fmt.Fprintf(t.sb, "%s ID: %s Size: %7v%s\n", intend, l.ID[:12], units.HumanSizeWithPrecision(float64(l.Size), 4), tags)
	}
}
