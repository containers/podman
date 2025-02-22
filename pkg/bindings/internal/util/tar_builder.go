package util

import (
	"archive/tar"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/containers/storage/pkg/fileutils"
	"github.com/containers/storage/pkg/ioutils"
	"github.com/hashicorp/go-multierror"
	gzip "github.com/klauspost/pgzip"
)

type Devino struct {
	Dev uint64
	Ino uint64
}

type TarBuilder struct {
	sources  []sourceMapping
	excludes []string
}

type sourceMapping struct {
	source string // Absolute path of the source directory/file
	target string // Custom path inside the tar archive
}

// NewTarBuilder returns a new TarBuilder
func NewTarBuilder() *TarBuilder {
	return &TarBuilder{
		sources:  []sourceMapping{},
		excludes: []string{},
	}
}

// Add adds a new source directory or file and the corresponding target inside the tar.
func (tb *TarBuilder) Add(source string, target string) error {
	absSource, err := filepath.Abs(source)
	if err != nil {
		return fmt.Errorf("failed to get absolute path for source: %v", err)
	}
	tb.sources = append(tb.sources, sourceMapping{source: absSource, target: target})
	return nil
}

// Exclude adds patterns to be excluded during tar creation.
func (tb *TarBuilder) Exclude(patterns ...string) {
	tb.excludes = append(tb.excludes, patterns...)
}

// Build generates the tarball and returns a ReadCloser for the tar stream.
func (tb *TarBuilder) Build() (io.ReadCloser, error) {
	if len(tb.sources) == 0 {
		return nil, fmt.Errorf("no source(s) added for tar creation")
	}

	pm, err := fileutils.NewPatternMatcher(tb.excludes)
	if err != nil {
		return nil, fmt.Errorf("processing excludes list %v: %w", tb.excludes, err)
	}

	pr, pw := io.Pipe()

	var merr *multierror.Error
	go func() {
		gw := gzip.NewWriter(pw)
		tw := tar.NewWriter(gw)

		defer pw.Close()
		defer gw.Close()
		defer tw.Close()

		seen := make(map[Devino]string)

		for _, src := range tb.sources {
			err = filepath.WalkDir(src.source, func(path string, dentry fs.DirEntry, err error) error {
				if err != nil {
					return err
				}

				// Build the relative path under the custom target path
				relPath, err := filepath.Rel(src.source, path)
				if err != nil {
					return err
				}
				targetPath := filepath.ToSlash(filepath.Join(src.target, relPath))

				// Check exclusion patterns
				if !filepath.IsAbs(targetPath) {
					excluded, err := pm.IsMatch(targetPath)
					if err != nil {
						return fmt.Errorf("checking if %q is excluded: %w", targetPath, err)
					}
					if excluded {
						return nil
					}
				}

				switch {
				case dentry.Type().IsRegular(): // Handle files
					info, err := dentry.Info()
					if err != nil {
						return err
					}
					di, isHardLink := CheckHardLink(info)
					if err != nil {
						return err
					}

					hdr, err := tar.FileInfoHeader(info, "")
					if err != nil {
						return err
					}
					hdr.Name = targetPath
					hdr.Uid, hdr.Gid = 0, 0
					orig, ok := seen[di]
					if ok {
						hdr.Typeflag = tar.TypeLink
						hdr.Linkname = orig
						hdr.Size = 0
						return tw.WriteHeader(hdr)
					}

					f, err := os.Open(path)
					if err != nil {
						return err
					}
					defer f.Close()

					if err := tw.WriteHeader(hdr); err != nil {
						return err
					}
					_, err = io.Copy(tw, f)
					if err == nil && isHardLink {
						seen[di] = targetPath
					}
					return err
				case dentry.IsDir(): // Handle directories
					info, err := dentry.Info()
					if err != nil {
						return err
					}
					hdr, lerr := tar.FileInfoHeader(info, targetPath)
					if lerr != nil {
						return lerr
					}
					hdr.Name = targetPath
					hdr.Uid, hdr.Gid = 0, 0
					return tw.WriteHeader(hdr)
				case dentry.Type()&os.ModeSymlink != 0: // Handle symlinks
					link, err := os.Readlink(path)
					if err != nil {
						return err
					}
					info, err := dentry.Info()
					if err != nil {
						return err
					}
					hdr, lerr := tar.FileInfoHeader(info, link)
					if lerr != nil {
						return lerr
					}
					hdr.Name = targetPath
					hdr.Uid, hdr.Gid = 0, 0
					return tw.WriteHeader(hdr)
				}
				return nil
			})

			if err != nil {
				merr = multierror.Append(merr, err)
			}
		}
	}()

	rc := ioutils.NewReadCloserWrapper(pr, func() error {
		if merr != nil {
			merr = multierror.Append(merr, pr.Close())
			return merr.ErrorOrNil()
		}
		return pr.Close()
	})

	return rc, nil
}
