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
	"bytes"
	"context"
	"crypto"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-openapi/strfmt"
	"github.com/go-openapi/swag/conv"

	"github.com/sigstore/rekor/pkg/generated/models"
	"github.com/sigstore/rekor/pkg/log"
	"github.com/sigstore/rekor/pkg/pki"
	"github.com/sigstore/rekor/pkg/pki/minisign"
	"github.com/sigstore/rekor/pkg/pki/pgp"
	"github.com/sigstore/rekor/pkg/pki/ssh"
	"github.com/sigstore/rekor/pkg/pki/x509"
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
	RekordObj models.RekordV001Schema
}

func (v V001Entry) APIVersion() string {
	return APIVERSION
}

func NewEntry() types.EntryImpl {
	return &V001Entry{}
}

func (v V001Entry) IndexKeys() ([]string, error) {
	var result []string

	af, err := pki.NewArtifactFactory(pki.Format(*v.RekordObj.Signature.Format))
	if err != nil {
		return nil, err
	}
	keyObj, err := af.NewPublicKey(bytes.NewReader(*v.RekordObj.Signature.PublicKey.Content))
	if err != nil {
		return nil, err
	}

	key, err := keyObj.CanonicalValue()
	if err != nil {
		log.Logger.Error(err)
	} else {
		keyHash := sha256.Sum256(key)
		result = append(result, strings.ToLower(hex.EncodeToString(keyHash[:])))
	}

	result = append(result, keyObj.Subjects()...)

	if v.RekordObj.Data.Hash != nil {
		hashKey := strings.ToLower(fmt.Sprintf("%s:%s", *v.RekordObj.Data.Hash.Algorithm, *v.RekordObj.Data.Hash.Value))
		result = append(result, hashKey)
	}

	return result, nil
}

func (v *V001Entry) Unmarshal(pe models.ProposedEntry) error {
	rekord, ok := pe.(*models.Rekord)
	if !ok {
		return errors.New("cannot unmarshal non Rekord v0.0.1 type")
	}

	if err := DecodeEntry(rekord.Spec, &v.RekordObj); err != nil {
		return err
	}

	// field validation
	if err := v.RekordObj.Validate(strfmt.Default); err != nil {
		return err
	}

	// cross field validation
	return v.validate()

}

// DecodeEntry performs direct JSON unmarshaling without reflection,
// equivalent to types.DecodeEntry but with better performance for Rekord v0.0.1.
// It avoids mutating the receiver on error.
func DecodeEntry(input any, output *models.RekordV001Schema) error {
	if output == nil {
		return fmt.Errorf("nil output *models.RekordV001Schema")
	}
	var m models.RekordV001Schema
	// Single switch including map[string]any fast path
	switch data := input.(type) {
	case map[string]any:
		mm := data
		if s, ok := mm["signature"].(map[string]any); ok {
			m.Signature = &models.RekordV001SchemaSignature{}
			if f, ok := s["format"].(string); ok {
				m.Signature.Format = &f
			}
			if c, ok := s["content"].(string); ok && c != "" {
				outb := make([]byte, base64.StdEncoding.DecodedLen(len(c)))
				n, err := base64.StdEncoding.Decode(outb, []byte(c))
				if err != nil {
					return fmt.Errorf("failed parsing base64 data for signature content: %w", err)
				}
				b := strfmt.Base64(outb[:n])
				m.Signature.Content = &b
			}
			if pk, ok := s["publicKey"].(map[string]any); ok {
				m.Signature.PublicKey = &models.RekordV001SchemaSignaturePublicKey{}
				if c, ok := pk["content"].(string); ok && c != "" {
					outb := make([]byte, base64.StdEncoding.DecodedLen(len(c)))
					n, err := base64.StdEncoding.Decode(outb, []byte(c))
					if err != nil {
						return fmt.Errorf("failed parsing base64 data for signature publicKey content: %w", err)
					}
					b := strfmt.Base64(outb[:n])
					m.Signature.PublicKey.Content = &b
				}
			}
		}
		if d, ok := mm["data"].(map[string]any); ok {
			m.Data = &models.RekordV001SchemaData{}
			if h, ok := d["hash"].(map[string]any); ok {
				m.Data.Hash = &models.RekordV001SchemaDataHash{}
				if alg, ok := h["algorithm"].(string); ok {
					m.Data.Hash.Algorithm = &alg
				}
				if val, ok := h["value"].(string); ok {
					m.Data.Hash.Value = &val
				}
			}
			if c, ok := d["content"].(string); ok && c != "" {
				outb := make([]byte, base64.StdEncoding.DecodedLen(len(c)))
				n, err := base64.StdEncoding.Decode(outb, []byte(c))
				if err != nil {
					return fmt.Errorf("failed parsing base64 data for data content: %w", err)
				}
				m.Data.Content = strfmt.Base64(outb[:n])
			}
		}
		*output = m
		return nil
	case *models.RekordV001Schema:
		if data == nil {
			return fmt.Errorf("nil *models.RekordV001Schema")
		}
		*output = *data
		return nil
	case models.RekordV001Schema:
		*output = data
		return nil
	default:
		return fmt.Errorf("unsupported input type %T for DecodeEntry", input)
	}
}

func (v *V001Entry) fetchExternalEntities(_ context.Context) (pki.PublicKey, pki.Signature, error) {
	af, err := pki.NewArtifactFactory(pki.Format(*v.RekordObj.Signature.Format))
	if err != nil {
		return nil, nil, err
	}

	// Hash computation
	hasher := sha256.New()
	if _, err := hasher.Write(v.RekordObj.Data.Content); err != nil {
		return nil, nil, &types.InputValidationError{Err: err}
	}
	computedSHA := hex.EncodeToString(hasher.Sum(nil))

	// Validate hash if provided
	if v.RekordObj.Data.Hash != nil && v.RekordObj.Data.Hash.Value != nil {
		oldSHA := conv.Value(v.RekordObj.Data.Hash.Value)
		if computedSHA != oldSHA {
			return nil, nil, &types.InputValidationError{Err: fmt.Errorf("SHA mismatch: %s != %s", computedSHA, oldSHA)}
		}
	}

	// Parse signature and key
	sigObj, err := af.NewSignature(bytes.NewReader(*v.RekordObj.Signature.Content))
	if err != nil {
		return nil, nil, &types.InputValidationError{Err: err}
	}

	keyObj, err := af.NewPublicKey(bytes.NewReader(*v.RekordObj.Signature.PublicKey.Content))
	if err != nil {
		return nil, nil, &types.InputValidationError{Err: err}
	}

	// Verify signature
	if err := sigObj.Verify(bytes.NewReader(v.RekordObj.Data.Content), keyObj); err != nil {
		return nil, nil, &types.InputValidationError{Err: err}
	}

	// Set computed hash if not provided
	if v.RekordObj.Data.Hash == nil {
		v.RekordObj.Data.Hash = &models.RekordV001SchemaDataHash{}
		v.RekordObj.Data.Hash.Algorithm = conv.Pointer(models.RekordV001SchemaDataHashAlgorithmSha256)
		v.RekordObj.Data.Hash.Value = conv.Pointer(computedSHA)
	}

	return keyObj, sigObj, nil
}

func (v *V001Entry) Canonicalize(ctx context.Context) ([]byte, error) {
	keyObj, sigObj, err := v.fetchExternalEntities(ctx)
	if err != nil {
		return nil, err
	}

	canonicalEntry := models.RekordV001Schema{}

	// need to canonicalize signature & key content
	canonicalEntry.Signature = &models.RekordV001SchemaSignature{}
	// signature URL (if known) is not set deliberately
	canonicalEntry.Signature.Format = v.RekordObj.Signature.Format

	var sigContent []byte
	sigContent, err = sigObj.CanonicalValue()
	if err != nil {
		return nil, err
	}
	canonicalEntry.Signature.Content = (*strfmt.Base64)(&sigContent)

	var pubKeyContent []byte
	canonicalEntry.Signature.PublicKey = &models.RekordV001SchemaSignaturePublicKey{}
	pubKeyContent, err = keyObj.CanonicalValue()
	if err != nil {
		return nil, err
	}
	canonicalEntry.Signature.PublicKey.Content = (*strfmt.Base64)(&pubKeyContent)

	canonicalEntry.Data = &models.RekordV001SchemaData{}
	canonicalEntry.Data.Hash = v.RekordObj.Data.Hash
	// data content is not set deliberately

	// wrap in valid object with kind and apiVersion set
	rekordObj := models.Rekord{}
	rekordObj.APIVersion = conv.Pointer(APIVERSION)
	rekordObj.Spec = &canonicalEntry

	v.RekordObj = canonicalEntry

	bytes, err := json.Marshal(&rekordObj)
	if err != nil {
		return nil, err
	}

	return bytes, nil
}

// validate performs cross-field validation for fields in object
func (v V001Entry) validate() error {
	sig := v.RekordObj.Signature
	if v.RekordObj.Signature == nil {
		return errors.New("missing signature")
	}
	if sig.Content == nil || len(*sig.Content) == 0 {
		return errors.New("'content' must be specified for signature")
	}

	key := sig.PublicKey
	if key == nil {
		return errors.New("missing public key")
	}
	if key.Content == nil || len(*key.Content) == 0 {
		return errors.New("'content' must be specified for publicKey")
	}

	data := v.RekordObj.Data
	if data == nil {
		return errors.New("missing data")
	}

	hash := data.Hash
	if hash != nil {
		// Rekord v0.0.1 schema enumerates sha256; enforce length accordingly.
		if hash.Value == nil || len(*hash.Value) != crypto.SHA256.Size()*2 {
			return errors.New("invalid value for hash")
		}
		if _, err := hex.DecodeString(*hash.Value); err != nil {
			return errors.New("invalid value for hash")
		}
	} else if len(data.Content) == 0 {
		return errors.New("'content' must be specified for data")
	}

	return nil
}

func (v V001Entry) CreateFromArtifactProperties(ctx context.Context, props types.ArtifactProperties) (models.ProposedEntry, error) {
	returnVal := models.Rekord{}
	re := V001Entry{}

	// we will need artifact, public-key, signature
	re.RekordObj.Data = &models.RekordV001SchemaData{}

	var err error
	artifactBytes := props.ArtifactBytes
	if len(artifactBytes) == 0 {
		var artifactReader io.ReadCloser
		if props.ArtifactPath == nil {
			return nil, errors.New("path to artifact file must be specified")
		}
		if props.ArtifactPath.IsAbs() {
			artifactReader, err = util.FileOrURLReadCloser(ctx, props.ArtifactPath.String(), nil)
			if err != nil {
				return nil, fmt.Errorf("error reading artifact file: %w", err)
			}
		} else {
			artifactReader, err = os.Open(filepath.Clean(props.ArtifactPath.Path))
			if err != nil {
				return nil, fmt.Errorf("error opening artifact file: %w", err)
			}
		}
		artifactBytes, err = io.ReadAll(artifactReader)
		if err != nil {
			return nil, fmt.Errorf("error reading artifact file: %w", err)
		}
	}
	re.RekordObj.Data.Content = strfmt.Base64(artifactBytes)

	re.RekordObj.Signature = &models.RekordV001SchemaSignature{}
	switch props.PKIFormat {
	case "pgp":
		re.RekordObj.Signature.Format = conv.Pointer(models.RekordV001SchemaSignatureFormatPgp)
	case "minisign":
		re.RekordObj.Signature.Format = conv.Pointer(models.RekordV001SchemaSignatureFormatMinisign)
	case "x509":
		re.RekordObj.Signature.Format = conv.Pointer(models.RekordV001SchemaSignatureFormatX509)
	case "ssh":
		re.RekordObj.Signature.Format = conv.Pointer(models.RekordV001SchemaSignatureFormatSSH)
	default:
		return nil, fmt.Errorf("unexpected format of public key: %s", props.PKIFormat)
	}
	sigBytes := props.SignatureBytes
	if len(sigBytes) == 0 {
		if props.SignaturePath == nil {
			return nil, errors.New("a detached signature must be provided")
		}
		sigBytes, err = os.ReadFile(filepath.Clean(props.SignaturePath.Path))
		if err != nil {
			return nil, fmt.Errorf("error reading signature file: %w", err)
		}
		re.RekordObj.Signature.Content = (*strfmt.Base64)(&sigBytes)
	} else {
		re.RekordObj.Signature.Content = (*strfmt.Base64)(&sigBytes)
	}

	re.RekordObj.Signature.PublicKey = &models.RekordV001SchemaSignaturePublicKey{}
	publicKeyBytes := props.PublicKeyBytes
	if len(publicKeyBytes) == 0 {
		if len(props.PublicKeyPaths) != 1 {
			return nil, errors.New("only one public key must be provided to verify detached signature")
		}
		keyBytes, err := os.ReadFile(filepath.Clean(props.PublicKeyPaths[0].Path))
		if err != nil {
			return nil, fmt.Errorf("error reading public key file: %w", err)
		}
		publicKeyBytes = append(publicKeyBytes, keyBytes)
	} else if len(publicKeyBytes) != 1 {
		return nil, errors.New("only one public key must be provided")
	}

	re.RekordObj.Signature.PublicKey.Content = (*strfmt.Base64)(&publicKeyBytes[0])

	if err := re.validate(); err != nil {
		return nil, err
	}

	if _, _, err := re.fetchExternalEntities(ctx); err != nil {
		return nil, fmt.Errorf("error retrieving external entities: %w", err)
	}

	returnVal.APIVersion = conv.Pointer(re.APIVersion())
	returnVal.Spec = re.RekordObj

	return &returnVal, nil
}

func (v V001Entry) Verifiers() ([]pki.PublicKey, error) {
	if v.RekordObj.Signature == nil || v.RekordObj.Signature.PublicKey == nil || v.RekordObj.Signature.PublicKey.Content == nil {
		return nil, errors.New("rekord v0.0.1 entry not initialized")
	}

	var key pki.PublicKey
	var err error
	switch f := *v.RekordObj.Signature.Format; f {
	case "x509":
		key, err = x509.NewPublicKey(bytes.NewReader(*v.RekordObj.Signature.PublicKey.Content))
	case "ssh":
		key, err = ssh.NewPublicKey(bytes.NewReader(*v.RekordObj.Signature.PublicKey.Content))
	case "pgp":
		key, err = pgp.NewPublicKey(bytes.NewReader(*v.RekordObj.Signature.PublicKey.Content))
	case "minisign":
		key, err = minisign.NewPublicKey(bytes.NewReader(*v.RekordObj.Signature.PublicKey.Content))
	default:
		return nil, fmt.Errorf("unexpected format of public key: %s", f)
	}
	if err != nil {
		return nil, err
	}
	return []pki.PublicKey{key}, nil
}

func (v V001Entry) ArtifactHash() (string, error) {
	if v.RekordObj.Data == nil || v.RekordObj.Data.Hash == nil || v.RekordObj.Data.Hash.Value == nil || v.RekordObj.Data.Hash.Algorithm == nil {
		return "", errors.New("rekord v0.0.1 entry not initialized")
	}
	return strings.ToLower(fmt.Sprintf("%s:%s", *v.RekordObj.Data.Hash.Algorithm, *v.RekordObj.Data.Hash.Value)), nil
}

func (v V001Entry) Insertable() (bool, error) {
	if v.RekordObj.Signature == nil {
		return false, errors.New("missing signature property")
	}
	if v.RekordObj.Signature.Content == nil || len(*v.RekordObj.Signature.Content) == 0 {
		return false, errors.New("missing signature content")
	}
	if v.RekordObj.Signature.PublicKey == nil {
		return false, errors.New("missing publicKey property")
	}
	if v.RekordObj.Signature.PublicKey.Content == nil || len(*v.RekordObj.Signature.PublicKey.Content) == 0 {
		return false, errors.New("missing publicKey content")
	}
	if v.RekordObj.Signature.Format == nil || len(*v.RekordObj.Signature.Format) == 0 {
		return false, errors.New("missing signature format")
	}

	if v.RekordObj.Data == nil {
		return false, errors.New("missing data property")
	}
	if len(v.RekordObj.Data.Content) == 0 {
		return false, errors.New("missing data content")
	}

	return true, nil
}
