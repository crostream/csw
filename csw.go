package csw

import (
	"encoding/json"
	"errors"
	"regexp"
)

// Format is the value of a .csw file's "csw" field.
const Format = 1

// MediaType is the media type of a .csw file; Extension its file extension.
const (
	MediaType = "application/vnd.crostream.widget+json"
	Extension = ".csw"
)

// API levels a widget can need: 1 is the base `sl` runtime, 2 adds
// sl.alert, sl.leave and sl.sfx, 3 adds sl.asset.
const (
	APIBase   = 1
	APIAlerts = 2
	APIAssets = 3
	// MaxAPI is the highest level this package knows.
	MaxAPI = APIAssets
)

// Limits of a .csw file.
const (
	MaxFileBytes   = 12 << 20 // the whole encoded file
	MaxAssets      = 64
	MaxAssetBytes  = 2 << 20 // one asset, decoded
	MaxAssetsBytes = 8 << 20 // all assets, decoded
	MaxAssetPath   = 128
	MaxIDLength    = 64
	maxAuthorName  = 80
	maxAuthorURL   = 200
	maxLicense     = 64
	maxCategory    = 32
)

var (
	// ErrFormat wraps the errors of reading a file that is not a .csw
	// document: bad JSON, an unknown field, a wrong format version or a
	// file over the size limit.
	ErrFormat = errors.New("not a valid .csw file")
	// ErrInvalid wraps the errors of a well-formed package that breaks a
	// rule (Package.Validate).
	ErrInvalid = errors.New("invalid widget package")
)

var (
	idRE       = regexp.MustCompile(`^[a-z][a-z0-9]*(\.[a-z0-9]+(-[a-z0-9]+)*)+$`)
	semverRE   = regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$`)
	categoryRE = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)
)

// OfficialNamespace starts the ID of every official CroStream widget.
const OfficialNamespace = "crostream."

// Package is one widget in the .csw format. Its JSON form is the file; the
// field order here is the file's key order.
type Package struct {
	CSW         int                        `json:"csw"`     // the format version, Format
	ID          string                     `json:"id"`      // reverse-DNS style, e.g. "crostream.alert-neon-burst"
	Version     string                     `json:"version"` // MAJOR.MINOR.PATCH
	API         int                        `json:"api"`     // the platform API level the widget needs
	Name        string                     `json:"name"`
	Description string                     `json:"description"`
	Author      Author                     `json:"author"`
	License     string                     `json:"license"` // an SPDX identifier or "proprietary"
	Category    string                     `json:"category"`
	Tags        []string                   `json:"tags"`
	Icon        string                     `json:"icon"`
	Size        Size                       `json:"size"`
	KeepAspect  bool                       `json:"keepAspect"`
	Fields      []Prop                     `json:"fields"`
	Defaults    json.RawMessage            `json:"defaults"` // a JSON object of the fields' initial values
	Data        []string                   `json:"data"`     // live data keys or prefixes the widget reads
	Sample      map[string]json.RawMessage `json:"sample"`   // preview data by data key
	Alert       *WidgetAlert               `json:"alert,omitempty"`
	Code        Code                       `json:"code"`
	Assets      map[string]Asset           `json:"assets,omitempty"`
}

// Author names who made a widget.
type Author struct {
	Name string `json:"name"`
	URL  string `json:"url,omitempty"`
}

// Code is a widget's HTML template, CSS and optional JS.
type Code struct {
	HTML string `json:"html"`
	CSS  string `json:"css"`
	JS   string `json:"js,omitempty"`
}

// Asset is one bundled file. Data is base64 in the JSON form.
type Asset struct {
	Type string `json:"type"` // a media type, see AssetTypes
	Data []byte `json:"data"`
}

// AssetTypes maps each allowed asset media type to the file extensions that
// may carry it.
func AssetTypes() map[string][]string {
	return map[string][]string{
		"image/png":     {".png"},
		"image/webp":    {".webp"},
		"image/svg+xml": {".svg"},
		"image/gif":     {".gif"},
		"image/jpeg":    {".jpg", ".jpeg"},
		"audio/ogg":     {".ogg"},
		"audio/mpeg":    {".mp3"},
		"audio/wav":     {".wav"},
		"font/woff2":    {".woff2"},
	}
}
