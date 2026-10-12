package csw

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"
)

// IndexFormat is the value of an index's "v" field.
const IndexFormat = 1

// Names in a gallery's dist folder (the root of the site's /v1).
const (
	IndexFile     = "index.json"
	IndexSigFile  = "index.json.sig"
	WidgetsDir    = "w"
	maxIndexBytes = 16 << 20
)

// ErrSignature is wrapped by a failed index signature check.
var ErrSignature = errors.New("index signature does not verify")

// Index is the gallery's catalog (/v1/index.json).
type Index struct {
	V         int           `json:"v"`
	Generated string        `json:"generated"` // RFC3339, UTC
	Widgets   []IndexWidget `json:"widgets"`   // sorted by ID
}

// IndexWidget is one widget of the catalog: the metadata of its latest
// version and the list of its versions.
type IndexWidget struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Author      string         `json:"author"`
	Category    string         `json:"category"`
	Tags        []string       `json:"tags"`
	Icon        string         `json:"icon"`
	Size        Size           `json:"size"`
	API         int            `json:"api"`    // of the latest version
	Latest      string         `json:"latest"` // the highest version
	Versions    []IndexVersion `json:"versions"`
	Featured    bool           `json:"featured"`
	Alert       *IndexAlert    `json:"alert"` // null unless the widget shows alerts
}

// IndexAlert is the part of a widget's alert block the catalog lists.
type IndexAlert struct {
	Events []string `json:"events"`
}

// IndexVersion is one published version, newest first in its list.
type IndexVersion struct {
	Version   string `json:"version"`
	URL       string `json:"url"` // relative to the index, w/<id>/<version>.csw
	SHA256    string `json:"sha256"`
	Bytes     int    `json:"bytes"`
	Published string `json:"published"` // RFC3339, UTC
	API       int    `json:"api"`
}

// IndexOptions are the inputs of BuildIndex besides the files.
type IndexOptions struct {
	// Featured lists the IDs to mark featured; each must exist.
	Featured []string
	// Now stamps the index and every version published for the first time.
	Now time.Time
	// Previous is the index being replaced, if any. Versions in it keep their
	// published time, and a version whose file changed is an error: published
	// versions are immutable.
	Previous *Index
}

// VersionPath returns the path of a version's file relative to the index.
func VersionPath(id, version string) string {
	return path.Join(WidgetsDir, id, version+Extension)
}

// BuildIndex scans dist/w/<id>/<version>.csw, validates every file and
// returns the catalog. Any stray file, bad package, file whose ID or version
// disagrees with its path, or changed published version is an error.
func BuildIndex(dist string, opts IndexOptions) (*Index, error) {
	ix := &Index{V: IndexFormat, Generated: stamp(opts.Now), Widgets: []IndexWidget{}}
	ids, err := os.ReadDir(filepath.Join(dist, WidgetsDir))
	if err != nil {
		return nil, fmt.Errorf("csw: index: %w", err)
	}
	for _, d := range ids {
		if !d.IsDir() {
			return nil, fmt.Errorf("csw: index: unexpected file %s/%s", WidgetsDir, d.Name())
		}
		wi, err := indexWidget(dist, d.Name(), opts)
		if err != nil {
			return nil, err
		}
		ix.Widgets = append(ix.Widgets, wi)
	}
	for _, id := range opts.Featured {
		i := slices.IndexFunc(ix.Widgets, func(w IndexWidget) bool { return w.ID == id })
		if i < 0 {
			return nil, fmt.Errorf("csw: index: featured widget %q is not in %s", id, dist)
		}
		ix.Widgets[i].Featured = true
	}
	return ix, nil
}

func stamp(t time.Time) string { return t.UTC().Truncate(time.Second).Format(time.RFC3339) }

// indexWidget builds the entry of the widget whose folder is dist/w/<id>.
func indexWidget(dist, id string, opts IndexOptions) (IndexWidget, error) {
	files, err := os.ReadDir(filepath.Join(dist, WidgetsDir, id))
	if err != nil {
		return IndexWidget{}, fmt.Errorf("csw: index: %w", err)
	}
	var (
		wi     IndexWidget
		latest *Package
	)
	for _, f := range files {
		ver, ok := strings.CutSuffix(f.Name(), Extension)
		if !ok || f.IsDir() {
			return IndexWidget{}, fmt.Errorf("csw: index: unexpected entry %s", path.Join(WidgetsDir, id, f.Name()))
		}
		data, err := readFileLimited(filepath.Join(dist, WidgetsDir, id, f.Name()), MaxFileBytes)
		if err != nil {
			return IndexWidget{}, err
		}
		p, err := decodeFile(data, id, ver)
		if err != nil {
			return IndexWidget{}, fmt.Errorf("csw: index: %s: %w", VersionPath(id, ver), err)
		}
		sum := sha256.Sum256(data)
		v := IndexVersion{
			Version: ver, URL: VersionPath(id, ver), SHA256: hex.EncodeToString(sum[:]), Bytes: len(data),
			Published: stamp(opts.Now), API: p.API,
		}
		if prev := previousVersion(opts.Previous, id, ver); prev != nil {
			if prev.SHA256 != v.SHA256 {
				return IndexWidget{}, fmt.Errorf("csw: index: %s changed since it was published (published versions are immutable; bump the version)", v.URL)
			}
			v.Published = prev.Published
		}
		wi.Versions = append(wi.Versions, v)
		if latest == nil || compareVersions(p.Version, latest.Version) > 0 {
			latest = p
		}
	}
	if latest == nil {
		return IndexWidget{}, fmt.Errorf("csw: index: %s/%s has no versions", WidgetsDir, id)
	}
	slices.SortFunc(wi.Versions, func(a, b IndexVersion) int { return compareVersions(b.Version, a.Version) })
	wi.ID, wi.Name, wi.Description, wi.Author = latest.ID, latest.Name, latest.Description, latest.Author.Name
	wi.Category, wi.Tags, wi.Icon, wi.Size = latest.Category, slices.Clone(latest.Tags), latest.Icon, latest.Size
	wi.API, wi.Latest = latest.API, latest.Version
	if wi.Tags == nil {
		wi.Tags = []string{}
	}
	if latest.Alert != nil {
		wi.Alert = &IndexAlert{Events: slices.Clone(latest.Alert.Events)}
	}
	return wi, nil
}

// decodeFile decodes and validates a file that must be widget id at ver.
func decodeFile(data []byte, id, ver string) (*Package, error) {
	p, err := Decode(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	if err := p.Validate(); err != nil {
		return nil, err
	}
	if p.ID != id || p.Version != ver {
		return nil, fmt.Errorf("csw: the file holds %s %s, not %s %s: %w", p.ID, p.Version, id, ver, ErrInvalid)
	}
	return p, nil
}

func previousVersion(ix *Index, id, ver string) *IndexVersion {
	if ix == nil {
		return nil
	}
	for i := range ix.Widgets {
		if ix.Widgets[i].ID != id {
			continue
		}
		for j := range ix.Widgets[i].Versions {
			if ix.Widgets[i].Versions[j].Version == ver {
				return &ix.Widgets[i].Versions[j]
			}
		}
	}
	return nil
}

// compareVersions orders MAJOR.MINOR.PATCH strings numerically.
func compareVersions(a, b string) int {
	pa, pb := strings.Split(a, "."), strings.Split(b, ".")
	for i := range min(len(pa), len(pb)) {
		x, _ := strconv.Atoi(pa[i])
		y, _ := strconv.Atoi(pb[i])
		if x != y {
			if x < y {
				return -1
			}
			return 1
		}
	}
	return len(pa) - len(pb)
}

func readFileLimited(name string, limit int64) ([]byte, error) {
	f, err := os.Open(name) //nolint:gosec // the caller names the file
	if err != nil {
		return nil, fmt.Errorf("csw: read %s: %w", name, err)
	}
	defer func() { _ = f.Close() }()
	b, err := io.ReadAll(io.LimitReader(f, limit+1))
	if err != nil {
		return nil, fmt.Errorf("csw: read %s: %w", name, err)
	}
	if int64(len(b)) > limit {
		return nil, fmt.Errorf("csw: read %s: larger than %d bytes", name, limit)
	}
	return b, nil
}

// Marshal returns the index's bytes: indented JSON ending in a newline.
// These exact bytes are what Sign signs and what the site must serve.
func (ix *Index) Marshal() ([]byte, error) { return marshal(ix) }

// DecodeIndex reads an index, rejecting unknown fields and other formats.
func DecodeIndex(data []byte) (*Index, error) {
	if len(data) > maxIndexBytes {
		return nil, fmt.Errorf("csw: index: %w: larger than %d bytes", ErrFormat, maxIndexBytes)
	}
	var ix Index
	if err := strictDecode(data, &ix); err != nil {
		return nil, fmt.Errorf("csw: index: %w: %w", ErrFormat, err)
	}
	if ix.V != IndexFormat {
		return nil, fmt.Errorf("csw: index: %w: unsupported version %d", ErrFormat, ix.V)
	}
	return &ix, nil
}

// SignIndex returns the base64 (standard) Ed25519 signature of the exact
// index bytes.
func SignIndex(data []byte, priv ed25519.PrivateKey) string {
	return base64.StdEncoding.EncodeToString(ed25519.Sign(priv, data))
}

// VerifyIndex checks sig (base64, surrounding space ignored) against the
// exact index bytes; a failure wraps ErrSignature.
func VerifyIndex(data []byte, sig string, pub ed25519.PublicKey) error {
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(sig))
	if err != nil {
		return fmt.Errorf("csw: %w: not base64: %w", ErrSignature, err)
	}
	if !ed25519.Verify(pub, data, raw) {
		return fmt.Errorf("csw: %w", ErrSignature)
	}
	return nil
}

// WriteIndex writes dist/index.json and dist/index.json.sig for ix, signed
// with priv, and returns the index bytes.
func WriteIndex(dist string, ix *Index, priv ed25519.PrivateKey) ([]byte, error) {
	data, err := ix.Marshal()
	if err != nil {
		return nil, err
	}
	if err := writeFile(filepath.Join(dist, IndexFile), data); err != nil {
		return nil, err
	}
	if err := writeFile(filepath.Join(dist, IndexSigFile), []byte(SignIndex(data, priv)+"\n")); err != nil {
		return nil, err
	}
	return data, nil
}

// ReadIndex reads dist/index.json without checking its signature. A missing
// file is an error wrapping fs.ErrNotExist.
func ReadIndex(dist string) (*Index, error) {
	data, err := readFileLimited(filepath.Join(dist, IndexFile), maxIndexBytes)
	if err != nil {
		return nil, err
	}
	return DecodeIndex(data)
}

// VerifyDist checks a dist folder against the public key: the signature on
// index.json, every listed file's presence, SHA-256, size, validity and
// identity, each widget's latest version and API level, and that no
// unlisted file sits in w/. It returns the number of version files checked
// and every problem found.
func VerifyDist(dist string, pub ed25519.PublicKey) (checked int, problems []error) {
	data, err := readFileLimited(filepath.Join(dist, IndexFile), maxIndexBytes)
	if err != nil {
		return 0, []error{err}
	}
	sig, err := readFileLimited(filepath.Join(dist, IndexSigFile), 4096)
	if err != nil {
		return 0, []error{err}
	}
	if err := VerifyIndex(data, string(sig), pub); err != nil {
		return 0, []error{err}
	}
	ix, err := DecodeIndex(data)
	if err != nil {
		return 0, []error{err}
	}
	listed := map[string]bool{}
	for i := range ix.Widgets {
		w := &ix.Widgets[i]
		listed[w.ID] = true
		for _, v := range w.Versions {
			checked++
			listed[path.Join(w.ID, v.Version+Extension)] = true
			if err := checkListed(dist, w, v); err != nil {
				problems = append(problems, err)
			}
		}
		if len(w.Versions) == 0 || w.Versions[0].Version != w.Latest {
			problems = append(problems, fmt.Errorf("csw: %s: latest %s is not its newest version", w.ID, w.Latest))
		}
	}
	return checked, append(problems, unlisted(dist, listed)...)
}

// checkListed checks one version file against its index entry.
func checkListed(dist string, w *IndexWidget, v IndexVersion) error {
	if v.URL != VersionPath(w.ID, v.Version) {
		return fmt.Errorf("csw: %s %s: url %q is not %s", w.ID, v.Version, v.URL, VersionPath(w.ID, v.Version))
	}
	data, err := readFileLimited(filepath.Join(dist, filepath.FromSlash(v.URL)), MaxFileBytes)
	if err != nil {
		return err
	}
	sum := sha256.Sum256(data)
	if hex.EncodeToString(sum[:]) != v.SHA256 {
		return fmt.Errorf("csw: %s: sha256 differs from the index", v.URL)
	}
	if len(data) != v.Bytes {
		return fmt.Errorf("csw: %s: %d bytes, the index says %d", v.URL, len(data), v.Bytes)
	}
	p, err := decodeFile(data, w.ID, v.Version)
	if err != nil {
		return fmt.Errorf("csw: %s: %w", v.URL, err)
	}
	if p.API != v.API {
		return fmt.Errorf("csw: %s: api %d, the index says %d", v.URL, p.API, v.API)
	}
	return nil
}

// unlisted reports files under dist/w that the index doesn't list (keys are
// "<id>" for folders and "<id>/<version>.csw" for files).
func unlisted(dist string, listed map[string]bool) []error {
	var out []error
	root := filepath.Join(dist, WidgetsDir)
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, p)
		rel = filepath.ToSlash(rel)
		if rel != "." && !listed[rel] {
			out = append(out, fmt.Errorf("csw: %s is not in the index", path.Join(WidgetsDir, rel)))
			if d.IsDir() {
				return fs.SkipDir
			}
		}
		return nil
	})
	if err != nil {
		out = append(out, fmt.Errorf("csw: scan %s: %w", root, err))
	}
	return out
}
