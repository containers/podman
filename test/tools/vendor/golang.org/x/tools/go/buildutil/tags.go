// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package buildutil

// This duplicated logic must be kept in sync with that from go build:
//   $GOROOT/src/cmd/go/internal/work/build.go (tagsFlag.Set)
//   $GOROOT/src/cmd/go/internal/base/flag.go (StringsFlag.Set)
//   $GOROOT/src/cmd/internal/quoted/quoted.go (isSpaceByte, Split)

import (
	"fmt"
	"strings"
)

const TagsFlagDoc = "a list of `build tags` to consider satisfied during the build. " +
	"For more information about build tags, see the description of " +
	"build constraints in the documentation for the go/build package"

// TagsFlag is an implementation of the flag.Value and flag.Getter interfaces that parses
// a flag value the same as go build's -tags flag and populates a []string slice.
//
// See $GOROOT/src/go/build/doc.go for description of build tags.
// See $GOROOT/src/cmd/go/doc.go for description of 'go build -tags' flag.
//
// Example:
//
//	flag.Var((*buildutil.TagsFlag)(&build.Default.BuildTags), "tags", buildutil.TagsFlagDoc)
type TagsFlag []string

func (v *TagsFlag) Set(s string) error {
	// See $GOROOT/src/cmd/go/internal/work/build.go (tagsFlag.Set)
	// For compatibility with Go 1.12 and earlier, allow "-tags='a b c'" or even just "-tags='a'".
	if strings.Contains(s, " ") || strings.Contains(s, "'") {
		var err error
		*v, err = splitQuotedFields(s)
		if *v == nil {
			*v = []string{}
		}
		return err
	}

	// Starting in Go 1.13, the -tags flag is a comma-separated list of build tags.
	*v = []string{}
	for _, s := range strings.Split(s, ",") {
		if s != "" {
			*v = append(*v, s)
		}
	}
	return nil
}

func (v *TagsFlag) Get() any { return *v }

func splitQuotedFields(s string) ([]string, error) {
	// See $GOROOT/src/cmd/internal/quoted/quoted.go (Split)
	// This must remain in sync with that logic.
	var f []string
	for len(s) > 0 {
		for len(s) > 0 && isSpaceByte(s[0]) {
			s = s[1:]
		}
		if len(s) == 0 {
			break
		}
		// Accepted quoted string. No unescaping inside.
		if s[0] == '"' || s[0] == '\'' {
			quote := s[0]
			s = s[1:]
			i := 0
			for i < len(s) && s[i] != quote {
				i++
			}
			if i >= len(s) {
				return nil, fmt.Errorf("unterminated %c string", quote)
			}
			f = append(f, s[:i])
			s = s[i+1:]
			continue
		}
		i := 0
		for i < len(s) && !isSpaceByte(s[i]) {
			i++
		}
		f = append(f, s[:i])
		s = s[i:]
	}
	return f, nil
}

func (v *TagsFlag) String() string {
	return "<tagsFlag>"
}

func isSpaceByte(c byte) bool {
	// See $GOROOT/src/cmd/internal/quoted/quoted.go (isSpaceByte, Split)
	// This list must remain in sync with that.
	return c == ' ' || c == '\t' || c == '\n' || c == '\r'
}
