package buildah

import (
	"hash"
	"strings"

	digest "github.com/opencontainers/go-digest"
)

type singleDigester struct {
	digester digest.Digester
	prefix   string
}

// CompositeDigester can compute a digest over multiple items.
type CompositeDigester struct {
	digesters []singleDigester
}

// Restart clears all state, so that the composite digester can start over.
func (c *CompositeDigester) Restart() {
	c.digesters = nil
}

// Start starts recording the digest for a new item.  The caller should call
// Hash() immediately after to retrieve the new io.Writer.
func (c *CompositeDigester) Start(prefix string) {
	prefix = strings.TrimSuffix(prefix, ":")
	c.digesters = append(c.digesters, singleDigester{digester: digest.Canonical.Digester(), prefix: prefix})
}

// Hash returns the hasher for the current item.
func (c *CompositeDigester) Hash() hash.Hash {
	num := len(c.digesters)
	if num == 0 {
		return nil
	}
	return c.digesters[num-1].digester.Hash()
}

// Digest returns the prefix and a composite digest over everything that's been
// digested.
func (c *CompositeDigester) Digest() (string, digest.Digest) {
	num := len(c.digesters)
	switch num {
	case 0:
		return "", ""
	case 1:
		return c.digesters[0].prefix, c.digesters[0].digester.Digest()
	default:
		content := ""
		for i, digester := range c.digesters {
			if i > 0 {
				content += ","
			}
			prefix := digester.prefix
			if digester.prefix != "" {
				digester.prefix += ":"
			}
			content += prefix + digester.digester.Digest().Encoded()
		}
		return "multi", digest.Canonical.FromString(content)
	}
}
