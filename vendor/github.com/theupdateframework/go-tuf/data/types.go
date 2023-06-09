package data

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/secure-systems-lab/go-securesystemslib/cjson"
)

type KeyType string

type KeyScheme string

type HashAlgorithm string

const (
	KeyIDLength = sha256.Size * 2

	KeyTypeEd25519           KeyType = "ed25519"
	KeyTypeECDSA_SHA2_P256   KeyType = "ecdsa-sha2-nistp256"
	KeyTypeRSASSA_PSS_SHA256 KeyType = "rsa"

	KeySchemeEd25519           KeyScheme = "ed25519"
	KeySchemeECDSA_SHA2_P256   KeyScheme = "ecdsa-sha2-nistp256"
	KeySchemeRSASSA_PSS_SHA256 KeyScheme = "rsassa-pss-sha256"

	HashAlgorithmSHA256 HashAlgorithm = "sha256"
	HashAlgorithmSHA512 HashAlgorithm = "sha512"
)

var (
	HashAlgorithms           = []HashAlgorithm{HashAlgorithmSHA256, HashAlgorithmSHA512}
	ErrPathsAndPathHashesSet = errors.New("tuf: failed validation of delegated target: paths and path_hash_prefixes are both set")
)

type Signed struct {
	Signed     json.RawMessage `json:"signed"`
	Signatures []Signature     `json:"signatures"`
}

type Signature struct {
	KeyID     string   `json:"keyid"`
	Signature HexBytes `json:"sig"`
}

type PublicKey struct {
	Type       KeyType         `json:"keytype"`
	Scheme     KeyScheme       `json:"scheme"`
	Algorithms []HashAlgorithm `json:"keyid_hash_algorithms,omitempty"`
	Value      json.RawMessage `json:"keyval"`

	ids    []string
	idOnce sync.Once
}

type PrivateKey struct {
	Type       KeyType         `json:"keytype"`
	Scheme     KeyScheme       `json:"scheme,omitempty"`
	Algorithms []HashAlgorithm `json:"keyid_hash_algorithms,omitempty"`
	Value      json.RawMessage `json:"keyval"`
}

func (k *PublicKey) IDs() []string {
	k.idOnce.Do(func() {
		data, err := cjson.EncodeCanonical(k)
		if err != nil {
			panic(fmt.Errorf("tuf: error creating key ID: %w", err))
		}
		digest := sha256.Sum256(data)
		k.ids = []string{hex.EncodeToString(digest[:])}
	})
	return k.ids
}

func (k *PublicKey) ContainsID(id string) bool {
	for _, keyid := range k.IDs() {
		if id == keyid {
			return true
		}
	}
	return false
}

func DefaultExpires(role string) time.Time {
	var t time.Time
	switch role {
	case "root":
		t = time.Now().AddDate(1, 0, 0)
	case "snapshot":
		t = time.Now().AddDate(0, 0, 7)
	case "timestamp":
		t = time.Now().AddDate(0, 0, 1)
	default:
		// targets and delegated targets
		t = time.Now().AddDate(0, 3, 0)
	}
	return t.UTC().Round(time.Second)
}

type Root struct {
	Type        string                `json:"_type"`
	SpecVersion string                `json:"spec_version"`
	Version     int64                 `json:"version"`
	Expires     time.Time             `json:"expires"`
	Keys        map[string]*PublicKey `json:"keys"`
	Roles       map[string]*Role      `json:"roles"`
	Custom      *json.RawMessage      `json:"custom,omitempty"`

	ConsistentSnapshot bool `json:"consistent_snapshot"`
}

func NewRoot() *Root {
	return &Root{
		Type:               "root",
		SpecVersion:        "1.0",
		Expires:            DefaultExpires("root"),
		Keys:               make(map[string]*PublicKey),
		Roles:              make(map[string]*Role),
		ConsistentSnapshot: true,
	}
}

func (r *Root) AddKey(key *PublicKey) bool {
	changed := false
	for _, id := range key.IDs() {
		if _, ok := r.Keys[id]; !ok {
			changed = true
			r.Keys[id] = key
		}
	}
	return changed
}

type Role struct {
	KeyIDs    []string `json:"keyids"`
	Threshold int      `json:"threshold"`
}

func (r *Role) AddKeyIDs(ids []string) bool {
	roleIDs := make(map[string]struct{})
	for _, id := range r.KeyIDs {
		roleIDs[id] = struct{}{}
	}
	changed := false
	for _, id := range ids {
		if _, ok := roleIDs[id]; !ok {
			changed = true
			r.KeyIDs = append(r.KeyIDs, id)
		}
	}
	return changed
}

type Files map[string]TargetFileMeta

type Hashes map[string]HexBytes

func (f Hashes) HashAlgorithms() []string {
	funcs := make([]string, 0, len(f))
	for name := range f {
		funcs = append(funcs, name)
	}
	return funcs
}

type metapathFileMeta struct {
	Length  int64            `json:"length,omitempty"`
	Hashes  Hashes           `json:"hashes,omitempty"`
	Version int64            `json:"version"`
	Custom  *json.RawMessage `json:"custom,omitempty"`
}

// SnapshotFileMeta is the meta field of a snapshot
// Note: Contains a `custom` field
type SnapshotFileMeta metapathFileMeta

type SnapshotFiles map[string]SnapshotFileMeta

type Snapshot struct {
	Type        string           `json:"_type"`
	SpecVersion string           `json:"spec_version"`
	Version     int64            `json:"version"`
	Expires     time.Time        `json:"expires"`
	Meta        SnapshotFiles    `json:"meta"`
	Custom      *json.RawMessage `json:"custom,omitempty"`
}

func NewSnapshot() *Snapshot {
	return &Snapshot{
		Type:        "snapshot",
		SpecVersion: "1.0",
		Expires:     DefaultExpires("snapshot"),
		Meta:        make(SnapshotFiles),
	}
}

type FileMeta struct {
	Length int64  `json:"length"`
	Hashes Hashes `json:"hashes"`
}

type TargetFiles map[string]TargetFileMeta

type TargetFileMeta struct {
	FileMeta
	Custom *json.RawMessage `json:"custom,omitempty"`
}

func (f TargetFileMeta) HashAlgorithms() []string {
	return f.FileMeta.Hashes.HashAlgorithms()
}

type Targets struct {
	Type        string           `json:"_type"`
	SpecVersion string           `json:"spec_version"`
	Version     int64            `json:"version"`
	Expires     time.Time        `json:"expires"`
	Targets     TargetFiles      `json:"targets"`
	Delegations *Delegations     `json:"delegations,omitempty"`
	Custom      *json.RawMessage `json:"custom,omitempty"`
}

// Delegations represents the edges from a parent Targets role to one or more
// delegated target roles. See spec v1.0.19 section 4.5.
type Delegations struct {
	Keys  map[string]*PublicKey `json:"keys"`
	Roles []DelegatedRole       `json:"roles"`
}

// DelegatedRole describes a delegated role, including what paths it is
// reponsible for. See spec v1.0.19 section 4.5.
type DelegatedRole struct {
	Name             string   `json:"name"`
	KeyIDs           []string `json:"keyids"`
	Threshold        int      `json:"threshold"`
	Terminating      bool     `json:"terminating"`
	PathHashPrefixes []string `json:"path_hash_prefixes,omitempty"`
	Paths            []string `json:"paths"`
}

// MatchesPath evaluates whether the path patterns or path hash prefixes match
// a given file. This determines whether a delegated role is responsible for
// signing and verifying the file.
func (d *DelegatedRole) MatchesPath(file string) (bool, error) {
	if err := d.validatePaths(); err != nil {
		return false, err
	}

	for _, pattern := range d.Paths {
		if matched, _ := path.Match(pattern, file); matched {
			return true, nil
		}
	}

	pathHash := PathHexDigest(file)
	for _, hashPrefix := range d.PathHashPrefixes {
		if strings.HasPrefix(pathHash, hashPrefix) {
			return true, nil
		}
	}

	return false, nil
}

// validatePaths enforces the spec
// https://theupdateframework.github.io/specification/v1.0.19/index.html#file-formats-targets
// 'role MUST specify only one of the "path_hash_prefixes" or "paths"'
// Marshalling and unmarshalling JSON will fail and return
// ErrPathsAndPathHashesSet if both fields are set and not empty.
func (d *DelegatedRole) validatePaths() error {
	if len(d.PathHashPrefixes) > 0 && len(d.Paths) > 0 {
		return ErrPathsAndPathHashesSet
	}

	return nil
}

// MarshalJSON is called when writing the struct to JSON. We validate prior to
// marshalling to ensure that an invalid delegated role can not be serialized
// to JSON.
func (d *DelegatedRole) MarshalJSON() ([]byte, error) {
	type delegatedRoleAlias DelegatedRole

	if err := d.validatePaths(); err != nil {
		return nil, err
	}

	return json.Marshal((*delegatedRoleAlias)(d))
}

// UnmarshalJSON is called when reading the struct from JSON. We validate once
// unmarshalled to ensure that an error is thrown if an invalid delegated role
// is read.
func (d *DelegatedRole) UnmarshalJSON(b []byte) error {
	type delegatedRoleAlias DelegatedRole

	// Prepare decoder
	dec := json.NewDecoder(bytes.NewReader(b))

	// Unmarshal delegated role
	if err := dec.Decode((*delegatedRoleAlias)(d)); err != nil {
		return err
	}

	return d.validatePaths()
}

func NewTargets() *Targets {
	return &Targets{
		Type:        "targets",
		SpecVersion: "1.0",
		Expires:     DefaultExpires("targets"),
		Targets:     make(TargetFiles),
	}
}

type TimestampFileMeta metapathFileMeta

type TimestampFiles map[string]TimestampFileMeta

type Timestamp struct {
	Type        string           `json:"_type"`
	SpecVersion string           `json:"spec_version"`
	Version     int64            `json:"version"`
	Expires     time.Time        `json:"expires"`
	Meta        TimestampFiles   `json:"meta"`
	Custom      *json.RawMessage `json:"custom,omitempty"`
}

func NewTimestamp() *Timestamp {
	return &Timestamp{
		Type:        "timestamp",
		SpecVersion: "1.0",
		Expires:     DefaultExpires("timestamp"),
		Meta:        make(TimestampFiles),
	}
}
