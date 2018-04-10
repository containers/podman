// +build !windows

package continuity

import (
	"bytes"
	_ "crypto/sha256"
	"errors"
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"syscall"
	"testing"

	"github.com/containerd/continuity/devices"
	"github.com/opencontainers/go-digest"
)

// Hard things:
//  1. Groups/gid - no standard library support.
//  2. xattrs - must choose package to provide this.
//  3. ADS - no clue where to start.

func TestWalkFS(t *testing.T) {
	rand.Seed(1)

	// Testing:
	// 1. Setup different files:
	//		- links
	//			- sibling directory - relative
	//			- sibling directory - absolute
	//			- parent directory - absolute
	//			- parent directory - relative
	//		- illegal links
	//			- parent directory - relative, out of root
	//			- parent directory - absolute, out of root
	//		- regular files
	//		- character devices
	//		- what about sticky bits?
	// 2. Build the manifest.
	// 3. Verify expected result.
	testResources := []dresource{
		{
			path: "a",
			mode: 0644,
		},
		{
			kind:   rhardlink,
			path:   "a-hardlink",
			target: "a",
		},
		{
			kind: rdirectory,
			path: "b",
			mode: 0755,
		},
		{
			kind:   rhardlink,
			path:   "b/a-hardlink",
			target: "a",
		},
		{
			path: "b/a",
			mode: 0600 | os.ModeSticky,
		},
		{
			kind: rdirectory,
			path: "c",
			mode: 0755,
		},
		{
			path: "c/a",
			mode: 0644,
		},
		{
			kind:   rrelsymlink,
			path:   "c/ca-relsymlink",
			mode:   0600,
			target: "a",
		},
		{
			kind:   rrelsymlink,
			path:   "c/a-relsymlink",
			mode:   0600,
			target: "../a",
		},
		{
			kind:   rabssymlink,
			path:   "c/a-abssymlink",
			mode:   0600,
			target: "a",
		},
		// TODO(stevvooe): Make sure we can test this case and get proper
		// errors when it is encountered.
		// {
		// 	// create a bad symlink and make sure we don't include it.
		// 	kind:   relsymlink,
		// 	path:   "c/a-badsymlink",
		// 	mode:   0600,
		// 	target: "../../..",
		// },

		// TODO(stevvooe): Must add tests for xattrs, with symlinks,
		// directorys and regular files.

		{
			kind: rnamedpipe,
			path: "fifo",
			mode: 0666 | os.ModeNamedPipe,
		},

		{
			kind: rdirectory,
			path: "/dev",
			mode: 0755,
		},

		// NOTE(stevvooe): Below here, we add a few simple character devices.
		// Block devices are untested but should be nearly the same as
		// character devices.
		// devNullResource,
		// devZeroResource,
	}

	root, err := ioutil.TempDir("", "continuity-test-")
	if err != nil {
		t.Fatalf("error creating temporary directory: %v", err)
	}

	defer os.RemoveAll(root)

	generateTestFiles(t, root, testResources)

	ctx, err := NewContext(root)
	if err != nil {
		t.Fatalf("error getting context: %v", err)
	}

	m, err := BuildManifest(ctx)
	if err != nil {
		t.Fatalf("error building manifest: %v", err)
	}

	var b bytes.Buffer
	MarshalText(&b, m)
	t.Log(b.String())

	// TODO(dmcgowan): always verify, currently hard links not supported
	//if err := VerifyManifest(ctx, m); err != nil {
	//	t.Fatalf("error verifying manifest: %v")
	//}

	expectedResources, err := expectedResourceList(root, testResources)
	if err != nil {
		// TODO(dmcgowan): update function to panic, this would mean test setup error
		t.Fatalf("error creating resource list: %v", err)
	}

	// Diff resources
	diff := diffResourceList(expectedResources, m.Resources)
	if diff.HasDiff() {
		t.Log("Resource list difference")
		for _, a := range diff.Additions {
			t.Logf("Unexpected resource: %#v", a)
		}
		for _, d := range diff.Deletions {
			t.Logf("Missing resource: %#v", d)
		}
		for _, u := range diff.Updates {
			t.Logf("Changed resource:\n\tExpected: %#v\n\tActual:   %#v", u.Original, u.Updated)
		}

		t.FailNow()
	}
}

// TODO(stevvooe): At this time, we have a nice testing framework to define
// and build resources. This will likely be a pre-cursor to the packages
// public interface.
type kind int

func (k kind) String() string {
	switch k {
	case rfile:
		return "file"
	case rdirectory:
		return "directory"
	case rhardlink:
		return "hardlink"
	case rchardev:
		return "chardev"
	case rnamedpipe:
		return "namedpipe"
	}

	panic(fmt.Sprintf("unknown kind: %v", int(k)))
}

const (
	rfile kind = iota
	rdirectory
	rhardlink
	rrelsymlink
	rabssymlink
	rchardev
	rnamedpipe
)

type dresource struct {
	kind         kind
	path         string
	mode         os.FileMode
	target       string // hard/soft link target
	digest       digest.Digest
	size         int
	uid          int64
	gid          int64
	major, minor int
}

func generateTestFiles(t *testing.T, root string, resources []dresource) {
	for i, resource := range resources {
		p := filepath.Join(root, resource.path)
		switch resource.kind {
		case rfile:
			size := rand.Intn(4 << 20)
			d := make([]byte, size)
			randomBytes(d)
			dgst := digest.FromBytes(d)
			resources[i].digest = dgst
			resources[i].size = size

			// this relies on the proper directory parent being defined.
			if err := ioutil.WriteFile(p, d, resource.mode); err != nil {
				t.Fatalf("error writing %q: %v", p, err)
			}
		case rdirectory:
			if err := os.Mkdir(p, resource.mode); err != nil {
				t.Fatalf("error creating directory %q: %v", p, err)
			}
		case rhardlink:
			target := filepath.Join(root, resource.target)
			if err := os.Link(target, p); err != nil {
				t.Fatalf("error creating hardlink: %v", err)
			}
		case rrelsymlink:
			if err := os.Symlink(resource.target, p); err != nil {
				t.Fatalf("error creating symlink: %v", err)
			}
		case rabssymlink:
			// for absolute links, we join with root.
			target := filepath.Join(root, resource.target)

			if err := os.Symlink(target, p); err != nil {
				t.Fatalf("error creating symlink: %v", err)
			}
		case rchardev, rnamedpipe:
			if err := devices.Mknod(p, resource.mode, resource.major, resource.minor); err != nil {
				t.Fatalf("error creating device %q: %v", p, err)
			}
		default:
			t.Fatalf("unknown resource type: %v", resource.kind)
		}

		st, err := os.Lstat(p)
		if err != nil {
			t.Fatalf("error statting after creation: %v", err)
		}
		resources[i].uid = int64(st.Sys().(*syscall.Stat_t).Uid)
		resources[i].gid = int64(st.Sys().(*syscall.Stat_t).Gid)
		resources[i].mode = st.Mode()

		// TODO: Readback and join xattr
	}

	// log the test root for future debugging
	if err := filepath.Walk(root, func(p string, fi os.FileInfo, err error) error {
		if fi.Mode()&os.ModeSymlink != 0 {
			target, err := os.Readlink(p)
			if err != nil {
				return err
			}
			t.Log(fi.Mode(), p, "->", target)
		} else {
			t.Log(fi.Mode(), p)
		}

		return nil
	}); err != nil {
		t.Fatalf("error walking created root: %v", err)
	}

	var b bytes.Buffer
	if err := tree(&b, root); err != nil {
		t.Fatalf("error running tree: %v", err)
	}
	t.Logf("\n%s", b.String())
}

func randomBytes(p []byte) {
	for i := range p {
		p[i] = byte(rand.Intn(1<<8 - 1))
	}
}

// expectedResourceList sorts the set of resources into the order
// expected in the manifest and collapses hardlinks
func expectedResourceList(root string, resources []dresource) ([]Resource, error) {
	resourceMap := map[string]Resource{}
	paths := []string{}
	for _, r := range resources {
		absPath := r.path
		if !filepath.IsAbs(absPath) {
			absPath = "/" + absPath
		}
		switch r.kind {
		case rfile:
			f := &regularFile{
				resource: resource{
					paths: []string{absPath},
					mode:  r.mode,
					uid:   r.uid,
					gid:   r.gid,
				},
				size:    int64(r.size),
				digests: []digest.Digest{r.digest},
			}
			resourceMap[absPath] = f
			paths = append(paths, absPath)
		case rdirectory:
			d := &directory{
				resource: resource{
					paths: []string{absPath},
					mode:  r.mode,
					uid:   r.uid,
					gid:   r.gid,
				},
			}
			resourceMap[absPath] = d
			paths = append(paths, absPath)
		case rhardlink:
			targetPath := r.target
			if !filepath.IsAbs(targetPath) {
				targetPath = "/" + targetPath
			}
			target, ok := resourceMap[targetPath]
			if !ok {
				return nil, errors.New("must specify target before hardlink for test resources")
			}
			rf, ok := target.(*regularFile)
			if !ok {
				return nil, errors.New("hardlink target must be regular file")
			}
			// TODO(dmcgowan): full merge
			rf.paths = append(rf.paths, absPath)
			// TODO(dmcgowan): check if first path is now different, changes source order and should update
			// resource map key, to avoid canonically ordered first should be regular file
			sort.Stable(sort.StringSlice(rf.paths))
		case rrelsymlink, rabssymlink:
			targetPath := r.target
			if r.kind == rabssymlink && !filepath.IsAbs(r.target) {
				// for absolute links, we join with root.
				targetPath = filepath.Join(root, targetPath)
			}
			s := &symLink{
				resource: resource{
					paths: []string{absPath},
					mode:  r.mode,
					uid:   r.uid,
					gid:   r.gid,
				},
				target: targetPath,
			}
			resourceMap[absPath] = s
			paths = append(paths, absPath)
		case rchardev:
			d := &device{
				resource: resource{
					paths: []string{absPath},
					mode:  r.mode,
					uid:   r.uid,
					gid:   r.gid,
				},
				major: uint64(r.major),
				minor: uint64(r.minor),
			}
			resourceMap[absPath] = d
			paths = append(paths, absPath)
		case rnamedpipe:
			p := &namedPipe{
				resource: resource{
					paths: []string{absPath},
					mode:  r.mode,
					uid:   r.uid,
					gid:   r.gid,
				},
			}
			resourceMap[absPath] = p
			paths = append(paths, absPath)
		default:
			return nil, fmt.Errorf("unknown resource type: %v", r.kind)
		}
	}

	if len(resourceMap) < len(paths) {
		return nil, errors.New("resource list has duplicated paths")
	}

	sort.Strings(paths)

	manifestResources := make([]Resource, len(paths))
	for i, p := range paths {
		manifestResources[i] = resourceMap[p]
	}

	return manifestResources, nil
}
