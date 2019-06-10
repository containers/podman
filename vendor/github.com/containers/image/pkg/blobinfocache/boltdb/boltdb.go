// Package boltdb implements a BlobInfoCache backed by BoltDB.
package boltdb

import (
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/containers/image/pkg/blobinfocache/internal/prioritize"
	"github.com/containers/image/types"
	bolt "github.com/etcd-io/bbolt"
	"github.com/opencontainers/go-digest"
	"github.com/sirupsen/logrus"
)

var (
	// NOTE: There is no versioning data inside the file; this is a “cache”, so on an incompatible format upgrade
	// we can simply start over with a different filename; update blobInfoCacheFilename.

	// FIXME: For CRI-O, does this need to hide information between different users?

	// uncompressedDigestBucket stores a mapping from any digest to an uncompressed digest.
	uncompressedDigestBucket = []byte("uncompressedDigest")
	// digestByUncompressedBucket stores a bucket per uncompressed digest, with the bucket containing a set of digests for that uncompressed digest
	// (as a set of key=digest, value="" pairs)
	digestByUncompressedBucket = []byte("digestByUncompressed")
	// knownLocationsBucket stores a nested structure of buckets, keyed by (transport name, scope string, blob digest), ultimately containing
	// a bucket of (opaque location reference, BinaryMarshaller-encoded time.Time value).
	knownLocationsBucket = []byte("knownLocations")
)

// Concurrency:
// See https://www.sqlite.org/src/artifact/c230a7a24?ln=994-1081 for all the issues with locks, which make it extremely
// difficult to use a single BoltDB file from multiple threads/goroutines inside a process.  So, we punt and only allow one at a time.

// pathLock contains a lock for a specific BoltDB database path.
type pathLock struct {
	refCount int64      // Number of threads/goroutines owning or waiting on this lock.  Protected by global pathLocksMutex, NOT by the mutex field below!
	mutex    sync.Mutex // Owned by the thread/goroutine allowed to access the BoltDB database.
}

var (
	// pathLocks contains a lock for each currently open file.
	// This must be global so that independently created instances of boltDBCache exclude each other.
	// The map is protected by pathLocksMutex.
	// FIXME? Should this be based on device:inode numbers instead of paths instead?
	pathLocks      = map[string]*pathLock{}
	pathLocksMutex = sync.Mutex{}
)

// lockPath obtains the pathLock for path.
// The caller must call unlockPath eventually.
func lockPath(path string) {
	pl := func() *pathLock { // A scope for defer
		pathLocksMutex.Lock()
		defer pathLocksMutex.Unlock()
		pl, ok := pathLocks[path]
		if ok {
			pl.refCount++
		} else {
			pl = &pathLock{refCount: 1, mutex: sync.Mutex{}}
			pathLocks[path] = pl
		}
		return pl
	}()
	pl.mutex.Lock()
}

// unlockPath releases the pathLock for path.
func unlockPath(path string) {
	pathLocksMutex.Lock()
	defer pathLocksMutex.Unlock()
	pl, ok := pathLocks[path]
	if !ok {
		// Should this return an error instead? BlobInfoCache ultimately ignores errors…
		panic(fmt.Sprintf("Internal error: unlocking nonexistent lock for path %s", path))
	}
	pl.mutex.Unlock()
	pl.refCount--
	if pl.refCount == 0 {
		delete(pathLocks, path)
	}
}

// cache is a BlobInfoCache implementation which uses a BoltDB file at the specified path.
//
// Note that we don’t keep the database open across operations, because that would lock the file and block any other
// users; instead, we need to open/close it for every single write or lookup.
type cache struct {
	path string
}

// New returns a BlobInfoCache implementation which uses a BoltDB file at path.
//
// Most users should call blobinfocache.DefaultCache instead.
func New(path string) types.BlobInfoCache {
	return &cache{path: path}
}

// view returns runs the specified fn within a read-only transaction on the database.
func (bdc *cache) view(fn func(tx *bolt.Tx) error) (retErr error) {
	// bolt.Open(bdc.path, 0600, &bolt.Options{ReadOnly: true}) will, if the file does not exist,
	// nevertheless create it, but with an O_RDONLY file descriptor, try to initialize it, and fail — while holding
	// a read lock, blocking any future writes.
	// Hence this preliminary check, which is RACY: Another process could remove the file
	// between the Lstat call and opening the database.
	if _, err := os.Lstat(bdc.path); err != nil && os.IsNotExist(err) {
		return err
	}

	lockPath(bdc.path)
	defer unlockPath(bdc.path)
	db, err := bolt.Open(bdc.path, 0600, &bolt.Options{ReadOnly: true})
	if err != nil {
		return err
	}
	defer func() {
		if err := db.Close(); retErr == nil && err != nil {
			retErr = err
		}
	}()

	return db.View(fn)
}

// update returns runs the specified fn within a read-write transaction on the database.
func (bdc *cache) update(fn func(tx *bolt.Tx) error) (retErr error) {
	lockPath(bdc.path)
	defer unlockPath(bdc.path)
	db, err := bolt.Open(bdc.path, 0600, nil)
	if err != nil {
		return err
	}
	defer func() {
		if err := db.Close(); retErr == nil && err != nil {
			retErr = err
		}
	}()

	return db.Update(fn)
}

// uncompressedDigest implements BlobInfoCache.UncompressedDigest within the provided read-only transaction.
func (bdc *cache) uncompressedDigest(tx *bolt.Tx, anyDigest digest.Digest) digest.Digest {
	if b := tx.Bucket(uncompressedDigestBucket); b != nil {
		if uncompressedBytes := b.Get([]byte(anyDigest.String())); uncompressedBytes != nil {
			d, err := digest.Parse(string(uncompressedBytes))
			if err == nil {
				return d
			}
			// FIXME? Log err (but throttle the log volume on repeated accesses)?
		}
	}
	// Presence in digestsByUncompressedBucket implies that anyDigest must already refer to an uncompressed digest.
	// This way we don't have to waste storage space with trivial (uncompressed, uncompressed) mappings
	// when we already record a (compressed, uncompressed) pair.
	if b := tx.Bucket(digestByUncompressedBucket); b != nil {
		if b = b.Bucket([]byte(anyDigest.String())); b != nil {
			c := b.Cursor()
			if k, _ := c.First(); k != nil { // The bucket is non-empty
				return anyDigest
			}
		}
	}
	return ""
}

// UncompressedDigest returns an uncompressed digest corresponding to anyDigest.
// May return anyDigest if it is known to be uncompressed.
// Returns "" if nothing is known about the digest (it may be compressed or uncompressed).
func (bdc *cache) UncompressedDigest(anyDigest digest.Digest) digest.Digest {
	var res digest.Digest
	if err := bdc.view(func(tx *bolt.Tx) error {
		res = bdc.uncompressedDigest(tx, anyDigest)
		return nil
	}); err != nil { // Including os.IsNotExist(err)
		return "" // FIXME? Log err (but throttle the log volume on repeated accesses)?
	}
	return res
}

// RecordDigestUncompressedPair records that the uncompressed version of anyDigest is uncompressed.
// It’s allowed for anyDigest == uncompressed.
// WARNING: Only call this for LOCALLY VERIFIED data; don’t record a digest pair just because some remote author claims so (e.g.
// because a manifest/config pair exists); otherwise the cache could be poisoned and allow substituting unexpected blobs.
// (Eventually, the DiffIDs in image config could detect the substitution, but that may be too late, and not all image formats contain that data.)
func (bdc *cache) RecordDigestUncompressedPair(anyDigest digest.Digest, uncompressed digest.Digest) {
	_ = bdc.update(func(tx *bolt.Tx) error {
		b, err := tx.CreateBucketIfNotExists(uncompressedDigestBucket)
		if err != nil {
			return err
		}
		key := []byte(anyDigest.String())
		if previousBytes := b.Get(key); previousBytes != nil {
			previous, err := digest.Parse(string(previousBytes))
			if err != nil {
				return err
			}
			if previous != uncompressed {
				logrus.Warnf("Uncompressed digest for blob %s previously recorded as %s, now %s", anyDigest, previous, uncompressed)
			}
		}
		if err := b.Put(key, []byte(uncompressed.String())); err != nil {
			return err
		}

		b, err = tx.CreateBucketIfNotExists(digestByUncompressedBucket)
		if err != nil {
			return err
		}
		b, err = b.CreateBucketIfNotExists([]byte(uncompressed.String()))
		if err != nil {
			return err
		}
		if err := b.Put([]byte(anyDigest.String()), []byte{}); err != nil { // Possibly writing the same []byte{} presence marker again.
			return err
		}
		return nil
	}) // FIXME? Log error (but throttle the log volume on repeated accesses)?
}

// RecordKnownLocation records that a blob with the specified digest exists within the specified (transport, scope) scope,
// and can be reused given the opaque location data.
func (bdc *cache) RecordKnownLocation(transport types.ImageTransport, scope types.BICTransportScope, blobDigest digest.Digest, location types.BICLocationReference) {
	_ = bdc.update(func(tx *bolt.Tx) error {
		b, err := tx.CreateBucketIfNotExists(knownLocationsBucket)
		if err != nil {
			return err
		}
		b, err = b.CreateBucketIfNotExists([]byte(transport.Name()))
		if err != nil {
			return err
		}
		b, err = b.CreateBucketIfNotExists([]byte(scope.Opaque))
		if err != nil {
			return err
		}
		b, err = b.CreateBucketIfNotExists([]byte(blobDigest.String()))
		if err != nil {
			return err
		}
		value, err := time.Now().MarshalBinary()
		if err != nil {
			return err
		}
		if err := b.Put([]byte(location.Opaque), value); err != nil { // Possibly overwriting an older entry.
			return err
		}
		return nil
	}) // FIXME? Log error (but throttle the log volume on repeated accesses)?
}

// appendReplacementCandiates creates prioritize.CandidateWithTime values for digest in scopeBucket, and returns the result of appending them to candidates.
func (bdc *cache) appendReplacementCandidates(candidates []prioritize.CandidateWithTime, scopeBucket *bolt.Bucket, digest digest.Digest) []prioritize.CandidateWithTime {
	b := scopeBucket.Bucket([]byte(digest.String()))
	if b == nil {
		return candidates
	}
	_ = b.ForEach(func(k, v []byte) error {
		t := time.Time{}
		if err := t.UnmarshalBinary(v); err != nil {
			return err
		}
		candidates = append(candidates, prioritize.CandidateWithTime{
			Candidate: types.BICReplacementCandidate{
				Digest:   digest,
				Location: types.BICLocationReference{Opaque: string(k)},
			},
			LastSeen: t,
		})
		return nil
	}) // FIXME? Log error (but throttle the log volume on repeated accesses)?
	return candidates
}

// CandidateLocations returns a prioritized, limited, number of blobs and their locations that could possibly be reused
// within the specified (transport scope) (if they still exist, which is not guaranteed).
//
// If !canSubstitute, the returned cadidates will match the submitted digest exactly; if canSubstitute,
// data from previous RecordDigestUncompressedPair calls is used to also look up variants of the blob which have the same
// uncompressed digest.
func (bdc *cache) CandidateLocations(transport types.ImageTransport, scope types.BICTransportScope, primaryDigest digest.Digest, canSubstitute bool) []types.BICReplacementCandidate {
	res := []prioritize.CandidateWithTime{}
	var uncompressedDigestValue digest.Digest // = ""
	if err := bdc.view(func(tx *bolt.Tx) error {
		scopeBucket := tx.Bucket(knownLocationsBucket)
		if scopeBucket == nil {
			return nil
		}
		scopeBucket = scopeBucket.Bucket([]byte(transport.Name()))
		if scopeBucket == nil {
			return nil
		}
		scopeBucket = scopeBucket.Bucket([]byte(scope.Opaque))
		if scopeBucket == nil {
			return nil
		}

		res = bdc.appendReplacementCandidates(res, scopeBucket, primaryDigest)
		if canSubstitute {
			if uncompressedDigestValue = bdc.uncompressedDigest(tx, primaryDigest); uncompressedDigestValue != "" {
				b := tx.Bucket(digestByUncompressedBucket)
				if b != nil {
					b = b.Bucket([]byte(uncompressedDigestValue.String()))
					if b != nil {
						if err := b.ForEach(func(k, _ []byte) error {
							d, err := digest.Parse(string(k))
							if err != nil {
								return err
							}
							if d != primaryDigest && d != uncompressedDigestValue {
								res = bdc.appendReplacementCandidates(res, scopeBucket, d)
							}
							return nil
						}); err != nil {
							return err
						}
					}
				}
				if uncompressedDigestValue != primaryDigest {
					res = bdc.appendReplacementCandidates(res, scopeBucket, uncompressedDigestValue)
				}
			}
		}
		return nil
	}); err != nil { // Including os.IsNotExist(err)
		return []types.BICReplacementCandidate{} // FIXME? Log err (but throttle the log volume on repeated accesses)?
	}

	return prioritize.DestructivelyPrioritizeReplacementCandidates(res, primaryDigest, uncompressedDigestValue)
}
