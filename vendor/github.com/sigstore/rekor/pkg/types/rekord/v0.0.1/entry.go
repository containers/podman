//
// Copyright 2021 The Sigstore Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package rekord

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"path/filepath"
	"strings"

	"github.com/asaskevich/govalidator"
	"github.com/go-openapi/strfmt"
	"github.com/go-openapi/swag"
	"golang.org/x/sync/errgroup"

	"github.com/sigstore/rekor/pkg/generated/models"
	"github.com/sigstore/rekor/pkg/log"
	"github.com/sigstore/rekor/pkg/pki"
	"github.com/sigstore/rekor/pkg/types"
	"github.com/sigstore/rekor/pkg/types/rekord"
	"github.com/sigstore/rekor/pkg/util"
)

const (
	APIVERSION = "0.0.1"
)

func init() {
	if err := rekord.VersionMap.SetEntryFactory(APIVERSION, NewEntry); err != nil {
		log.Logger.Panic(err)
	}
}

type V001Entry struct {
	RekordObj               models.RekordV001Schema
	fetchedExternalEntities bool
	keyObj                  pki.PublicKey
	sigObj                  pki.Signature
}

func (v V001Entry) APIVersion() string {
	return APIVERSION
}

func NewEntry() types.EntryImpl {
	return &V001Entry{}
}

func (v V001Entry) IndexKeys() []string {
	var result []string

	if v.HasExternalEntities() {
		if err := v.FetchExternalEntities(context.Background()); err != nil {
			log.Logger.Error(err)
			return result
		}
	}

	key, err := v.keyObj.CanonicalValue()
	if err != nil {
		log.Logger.Error(err)
	} else {
		keyHash := sha256.Sum256(key)
		result = append(result, strings.ToLower(hex.EncodeToString(keyHash[:])))
	}

	result = append(result, v.keyObj.EmailAddresses()...)

	if v.RekordObj.Data.Hash != nil {
		hashKey := strings.ToLower(fmt.Sprintf("%s:%s", *v.RekordObj.Data.Hash.Algorithm, *v.RekordObj.Data.Hash.Value))
		result = append(result, hashKey)
	}

	return result
}

func (v *V001Entry) Unmarshal(pe models.ProposedEntry) error {
	rekord, ok := pe.(*models.Rekord)
	if !ok {
		return errors.New("cannot unmarshal non Rekord v0.0.1 type")
	}

	if err := types.DecodeEntry(rekord.Spec, &v.RekordObj); err != nil {
		return err
	}

	// field validation
	if err := v.RekordObj.Validate(strfmt.Default); err != nil {
		return err
	}
	// cross field validation
	return v.validate()

}

func (v V001Entry) HasExternalEntities() bool {
	if v.fetchedExternalEntities {
		return false
	}

	if v.RekordObj.Data != nil && v.RekordObj.Data.URL.String() != "" {
		return true
	}
	if v.RekordObj.Signature != nil && v.RekordObj.Signature.URL.String() != "" {
		return true
	}
	if v.RekordObj.Signature != nil && v.RekordObj.Signature.PublicKey != nil && v.RekordObj.Signature.PublicKey.URL.String() != "" {
		return true
	}
	return false
}

func (v *V001Entry) FetchExternalEntities(ctx context.Context) error {
	if v.fetchedExternalEntities {
		return nil
	}

	if err := v.validate(); err != nil {
		return types.ValidationError(err)
	}

	g, ctx := errgroup.WithContext(ctx)

	hashR, hashW := io.Pipe()
	sigR, sigW := io.Pipe()
	defer hashR.Close()
	defer sigR.Close()

	closePipesOnError := func(err error) error {
		pipeReaders := []*io.PipeReader{hashR, sigR}
		pipeWriters := []*io.PipeWriter{hashW, sigW}
		for idx := range pipeReaders {
			if e := pipeReaders[idx].CloseWithError(err); e != nil {
				log.Logger.Error(fmt.Errorf("error closing pipe: %w", e))
			}
			if e := pipeWriters[idx].CloseWithError(err); e != nil {
				log.Logger.Error(fmt.Errorf("error closing pipe: %w", e))
			}
		}
		return err
	}

	oldSHA := ""
	if v.RekordObj.Data.Hash != nil && v.RekordObj.Data.Hash.Value != nil {
		oldSHA = swag.StringValue(v.RekordObj.Data.Hash.Value)
	}
	artifactFactory, err := pki.NewArtifactFactory(pki.Format(v.RekordObj.Signature.Format))
	if err != nil {
		return types.ValidationError(err)
	}

	g.Go(func() error {
		defer hashW.Close()
		defer sigW.Close()

		dataReadCloser, err := util.FileOrURLReadCloser(ctx, v.RekordObj.Data.URL.String(), v.RekordObj.Data.Content)
		if err != nil {
			return closePipesOnError(err)
		}
		defer dataReadCloser.Close()

		/* #nosec G110 */
		if _, err := io.Copy(io.MultiWriter(hashW, sigW), dataReadCloser); err != nil {
			return closePipesOnError(err)
		}
		return nil
	})

	hashResult := make(chan string)

	g.Go(func() error {
		defer close(hashResult)
		hasher := sha256.New()

		if _, err := io.Copy(hasher, hashR); err != nil {
			return closePipesOnError(err)
		}

		computedSHA := hex.EncodeToString(hasher.Sum(nil))
		if oldSHA != "" && computedSHA != oldSHA {
			return closePipesOnError(types.ValidationError(fmt.Errorf("SHA mismatch: %s != %s", computedSHA, oldSHA)))
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case hashResult <- computedSHA:
			return nil
		}
	})

	sigResult := make(chan pki.Signature)

	g.Go(func() error {
		defer close(sigResult)

		sigReadCloser, err := util.FileOrURLReadCloser(ctx, v.RekordObj.Signature.URL.String(),
			v.RekordObj.Signature.Content)
		if err != nil {
			return closePipesOnError(err)
		}
		defer sigReadCloser.Close()

		signature, err := artifactFactory.NewSignature(sigReadCloser)
		if err != nil {
			return closePipesOnError(types.ValidationError(err))
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case sigResult <- signature:
			return nil
		}
	})

	keyResult := make(chan pki.PublicKey)

	g.Go(func() error {
		defer close(keyResult)

		keyReadCloser, err := util.FileOrURLReadCloser(ctx, v.RekordObj.Signature.PublicKey.URL.String(),
			v.RekordObj.Signature.PublicKey.Content)
		if err != nil {
			return closePipesOnError(err)
		}
		defer keyReadCloser.Close()

		key, err := artifactFactory.NewPublicKey(keyReadCloser)
		if err != nil {
			return closePipesOnError(types.ValidationError(err))
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case keyResult <- key:
			return nil
		}
	})

	g.Go(func() error {
		v.keyObj, v.sigObj = <-keyResult, <-sigResult

		if v.keyObj == nil || v.sigObj == nil {
			return closePipesOnError(errors.New("failed to read signature or public key"))
		}

		var err error
		if err = v.sigObj.Verify(sigR, v.keyObj); err != nil {
			return closePipesOnError(types.ValidationError(err))
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			return nil
		}
	})

	computedSHA := <-hashResult

	if err := g.Wait(); err != nil {
		return err
	}

	// if we get here, all goroutines succeeded without error
	if oldSHA == "" {
		v.RekordObj.Data.Hash = &models.RekordV001SchemaDataHash{}
		v.RekordObj.Data.Hash.Algorithm = swag.String(models.RekordV001SchemaDataHashAlgorithmSha256)
		v.RekordObj.Data.Hash.Value = swag.String(computedSHA)
	}

	v.fetchedExternalEntities = true
	return nil
}

func (v *V001Entry) Canonicalize(ctx context.Context) ([]byte, error) {
	if err := v.FetchExternalEntities(ctx); err != nil {
		return nil, err
	}
	if v.sigObj == nil {
		return nil, errors.New("signature object not initialized before canonicalization")
	}
	if v.keyObj == nil {
		return nil, errors.New("key object not initialized before canonicalization")
	}

	canonicalEntry := models.RekordV001Schema{}
	canonicalEntry.ExtraData = v.RekordObj.ExtraData

	// need to canonicalize signature & key content
	canonicalEntry.Signature = &models.RekordV001SchemaSignature{}
	// signature URL (if known) is not set deliberately
	canonicalEntry.Signature.Format = v.RekordObj.Signature.Format

	var err error
	canonicalEntry.Signature.Content, err = v.sigObj.CanonicalValue()
	if err != nil {
		return nil, err
	}

	// key URL (if known) is not set deliberately
	canonicalEntry.Signature.PublicKey = &models.RekordV001SchemaSignaturePublicKey{}
	canonicalEntry.Signature.PublicKey.Content, err = v.keyObj.CanonicalValue()
	if err != nil {
		return nil, err
	}

	canonicalEntry.Data = &models.RekordV001SchemaData{}
	canonicalEntry.Data.Hash = v.RekordObj.Data.Hash
	// data content is not set deliberately

	// ExtraData is copied through unfiltered
	canonicalEntry.ExtraData = v.RekordObj.ExtraData

	// wrap in valid object with kind and apiVersion set
	rekordObj := models.Rekord{}
	rekordObj.APIVersion = swag.String(APIVERSION)
	rekordObj.Spec = &canonicalEntry

	bytes, err := json.Marshal(&rekordObj)
	if err != nil {
		return nil, err
	}

	return bytes, nil
}

// validate performs cross-field validation for fields in object
func (v V001Entry) validate() error {
	sig := v.RekordObj.Signature
	if sig == nil {
		return errors.New("missing signature")
	}
	if len(sig.Content) == 0 && sig.URL.String() == "" {
		return errors.New("one of 'content' or 'url' must be specified for signature")
	}

	key := sig.PublicKey
	if key == nil {
		return errors.New("missing public key")
	}
	if len(key.Content) == 0 && key.URL.String() == "" {
		return errors.New("one of 'content' or 'url' must be specified for publicKey")
	}

	data := v.RekordObj.Data
	if data == nil {
		return errors.New("missing data")
	}

	hash := data.Hash
	if hash != nil {
		if !govalidator.IsHash(swag.StringValue(hash.Value), swag.StringValue(hash.Algorithm)) {
			return errors.New("invalid value for hash")
		}
	} else if len(data.Content) == 0 && data.URL.String() == "" {
		return errors.New("one of 'content' or 'url' must be specified for data")
	}

	return nil
}

func (v V001Entry) Attestation() (string, []byte) {
	return "", nil
}

func (v V001Entry) CreateFromArtifactProperties(ctx context.Context, props types.ArtifactProperties) (models.ProposedEntry, error) {
	returnVal := models.Rekord{}
	re := V001Entry{}

	// we will need artifact, public-key, signature
	re.RekordObj.Data = &models.RekordV001SchemaData{}

	var err error
	artifactBytes := props.ArtifactBytes
	if artifactBytes == nil {
		if props.ArtifactPath == nil {
			return nil, errors.New("path to artifact (file or URL) must be specified")
		}
		if props.ArtifactPath.IsAbs() {
			re.RekordObj.Data.URL = strfmt.URI(props.ArtifactPath.String())
			if props.ArtifactHash != "" {
				re.RekordObj.Data.Hash = &models.RekordV001SchemaDataHash{
					Algorithm: swag.String(models.RekordV001SchemaDataHashAlgorithmSha256),
					Value:     swag.String(props.ArtifactHash),
				}
			}
		} else {
			artifactBytes, err := ioutil.ReadFile(filepath.Clean(props.ArtifactPath.Path))
			if err != nil {
				return nil, fmt.Errorf("error reading artifact file: %w", err)
			}
			re.RekordObj.Data.Content = strfmt.Base64(artifactBytes)
		}
	} else {
		re.RekordObj.Data.Content = strfmt.Base64(artifactBytes)
	}

	re.RekordObj.Signature = &models.RekordV001SchemaSignature{}
	switch props.PKIFormat {
	case "pgp":
		re.RekordObj.Signature.Format = models.RekordV001SchemaSignatureFormatPgp
	case "minisign":
		re.RekordObj.Signature.Format = models.RekordV001SchemaSignatureFormatMinisign
	case "x509":
		re.RekordObj.Signature.Format = models.RekordV001SchemaSignatureFormatX509
	case "ssh":
		re.RekordObj.Signature.Format = models.RekordV001SchemaSignatureFormatSSH
	}
	sigBytes := props.SignatureBytes
	if sigBytes == nil {
		if props.SignaturePath == nil {
			return nil, errors.New("a detached signature must be provided")
		}
		if props.SignaturePath.IsAbs() {
			re.RekordObj.Signature.URL = strfmt.URI(props.SignaturePath.String())
		} else {
			sigBytes, err = ioutil.ReadFile(filepath.Clean(props.SignaturePath.Path))
			if err != nil {
				return nil, fmt.Errorf("error reading signature file: %w", err)
			}
			re.RekordObj.Signature.Content = strfmt.Base64(sigBytes)
		}
	} else {
		re.RekordObj.Signature.Content = strfmt.Base64(sigBytes)
	}

	re.RekordObj.Signature.PublicKey = &models.RekordV001SchemaSignaturePublicKey{}
	publicKeyBytes := props.PublicKeyBytes
	if publicKeyBytes == nil {
		if props.PublicKeyPath == nil {
			return nil, errors.New("public key must be provided to verify detached signature")
		}
		if props.PublicKeyPath.IsAbs() {
			re.RekordObj.Signature.PublicKey.URL = strfmt.URI(props.PublicKeyPath.String())
		} else {
			publicKeyBytes, err = ioutil.ReadFile(filepath.Clean(props.PublicKeyPath.Path))
			if err != nil {
				return nil, fmt.Errorf("error reading public key file: %w", err)
			}
			re.RekordObj.Signature.PublicKey.Content = strfmt.Base64(publicKeyBytes)
		}
	} else {
		re.RekordObj.Signature.PublicKey.Content = strfmt.Base64(publicKeyBytes)
	}

	if err := re.validate(); err != nil {
		return nil, err
	}

	if re.HasExternalEntities() {
		if err := re.FetchExternalEntities(ctx); err != nil {
			return nil, fmt.Errorf("error retrieving external entities: %v", err)
		}
	}

	returnVal.APIVersion = swag.String(re.APIVersion())
	returnVal.Spec = re.RekordObj

	return &returnVal, nil
}
