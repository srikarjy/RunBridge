package integrity

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sort"
	"strings"
)

var ErrInvalidManifest = errors.New("invalid artifact manifest")

type Artifact struct {
	Path   string `json:"path"`
	Digest string `json:"digest"`
}
type Manifest struct {
	Artifacts []Artifact `json:"artifacts"`
}

func NewManifest(artifacts []Artifact) (Manifest, error) {
	if len(artifacts) == 0 {
		return Manifest{}, ErrInvalidManifest
	}
	copyArtifacts := append([]Artifact(nil), artifacts...)
	seen := make(map[string]struct{}, len(copyArtifacts))
	for index := range copyArtifacts {
		copyArtifacts[index].Path = strings.TrimSpace(copyArtifacts[index].Path)
		copyArtifacts[index].Digest = strings.TrimSpace(copyArtifacts[index].Digest)
		if copyArtifacts[index].Path == "" || copyArtifacts[index].Digest == "" {
			return Manifest{}, ErrInvalidManifest
		}
		if _, exists := seen[copyArtifacts[index].Path]; exists {
			return Manifest{}, ErrInvalidManifest
		}
		seen[copyArtifacts[index].Path] = struct{}{}
	}
	sort.Slice(copyArtifacts, func(left, right int) bool { return copyArtifacts[left].Path < copyArtifacts[right].Path })
	return Manifest{Artifacts: copyArtifacts}, nil
}

func (manifest Manifest) CanonicalBytes() []byte { bytes, _ := json.Marshal(manifest); return bytes }
func (manifest Manifest) DigestHex() string {
	digest := sha256.Sum256(manifest.CanonicalBytes())
	return hex.EncodeToString(digest[:])
}
