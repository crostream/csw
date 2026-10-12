package csw

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

// manifest is a Package without its code and assets: the metadata, as the
// gallery source folders keep it in manifest.json. Its field order is the
// file's key order.
type manifest struct {
	CSW         int                        `json:"csw"`
	ID          string                     `json:"id"`
	Version     string                     `json:"version"`
	API         int                        `json:"api"`
	Name        string                     `json:"name"`
	Description string                     `json:"description"`
	Author      Author                     `json:"author"`
	License     string                     `json:"license"`
	Category    string                     `json:"category"`
	Tags        []string                   `json:"tags"`
	Icon        string                     `json:"icon"`
	Size        Size                       `json:"size"`
	KeepAspect  bool                       `json:"keepAspect"`
	Fields      []Prop                     `json:"fields"`
	Defaults    json.RawMessage            `json:"defaults"`
	Data        []string                   `json:"data"`
	Sample      map[string]json.RawMessage `json:"sample"`
	Alert       *WidgetAlert               `json:"alert,omitempty"`
}

// wire is the .csw file: the manifest followed by the code and assets.
type wire struct {
	manifest
	Code   Code             `json:"code"`
	Assets map[string]Asset `json:"assets,omitempty"`
}

// canonical returns p with nil lists and maps made empty and every raw JSON
// value rewritten with sorted keys, so equal packages encode to equal bytes.
func canonical(p *Package) (Package, error) {
	c := *p
	if c.Tags == nil {
		c.Tags = []string{}
	}
	if c.Fields == nil {
		c.Fields = []Prop{}
	}
	if c.Data == nil {
		c.Data = []string{}
	}
	if c.Sample == nil {
		c.Sample = map[string]json.RawMessage{}
	}
	if len(c.Defaults) == 0 || string(c.Defaults) == "null" {
		c.Defaults = json.RawMessage("{}")
	}
	var err error
	if c.Defaults, err = sortedJSON(c.Defaults); err != nil {
		return Package{}, fmt.Errorf("csw: defaults: %w", err)
	}
	sample := make(map[string]json.RawMessage, len(c.Sample))
	for k, v := range c.Sample {
		if sample[k], err = sortedJSON(v); err != nil {
			return Package{}, fmt.Errorf("csw: sample %q: %w", k, err)
		}
	}
	c.Sample = sample
	return c, nil
}

// sortedJSON rewrites raw JSON compactly with object keys sorted, keeping
// numbers as written.
func sortedJSON(raw json.RawMessage) (json.RawMessage, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return json.RawMessage(bytes.TrimRight(buf.Bytes(), "\n")), nil
}

// Encode returns the .csw file for p: JSON with a fixed key order, sorted
// map keys, two-space indentation and a trailing newline. The same package
// always encodes to the same bytes, so the file's SHA-256 is reproducible.
// Encode does not validate p; it fails only if the result would exceed
// MaxFileBytes or p holds invalid JSON.
func Encode(p *Package) ([]byte, error) {
	c, err := canonical(p)
	if err != nil {
		return nil, err
	}
	out, err := marshal(wire{manifest: toManifest(&c), Code: c.Code, Assets: c.Assets})
	if err != nil {
		return nil, err
	}
	if len(out) > MaxFileBytes {
		return nil, fmt.Errorf("csw: encode: the file is %d bytes, over the %d limit: %w", len(out), MaxFileBytes, ErrInvalid)
	}
	return out, nil
}

// marshal encodes v as indented JSON without HTML escaping, ending in a
// newline.
func marshal(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		return nil, fmt.Errorf("csw: encode: %w", err)
	}
	return buf.Bytes(), nil
}

// Decode reads one .csw file. It rejects unknown fields, trailing data, a
// file over MaxFileBytes and a format version other than Format; every such
// error wraps ErrFormat. It does not apply the package rules: call Validate.
func Decode(r io.Reader) (*Package, error) {
	data, err := io.ReadAll(io.LimitReader(r, MaxFileBytes+1))
	if err != nil {
		return nil, fmt.Errorf("csw: read: %w", err)
	}
	if len(data) > MaxFileBytes {
		return nil, fmt.Errorf("csw: %w: larger than %d bytes", ErrFormat, MaxFileBytes)
	}
	var w wire
	if err := strictDecode(data, &w); err != nil {
		return nil, fmt.Errorf("csw: %w: %w", ErrFormat, err)
	}
	if w.CSW != Format {
		return nil, fmt.Errorf("csw: %w: unsupported format version %d (want %d)", ErrFormat, w.CSW, Format)
	}
	p := fromManifest(&w.manifest)
	p.Code, p.Assets = w.Code, w.Assets
	return &p, nil
}

// strictDecode decodes exactly one JSON value into v, rejecting unknown
// fields and trailing data.
func strictDecode(data []byte, v any) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return err
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return errors.New("more than one JSON value")
	}
	return nil
}

// toManifest returns the metadata of p.
func toManifest(p *Package) manifest {
	return manifest{
		CSW: p.CSW, ID: p.ID, Version: p.Version, API: p.API, Name: p.Name, Description: p.Description,
		Author: p.Author, License: p.License, Category: p.Category, Tags: p.Tags, Icon: p.Icon, Size: p.Size,
		KeepAspect: p.KeepAspect, Fields: p.Fields, Defaults: p.Defaults, Data: p.Data, Sample: p.Sample,
		Alert: p.Alert,
	}
}

// fromManifest returns a package with m's metadata and no code or assets.
func fromManifest(m *manifest) Package {
	return Package{
		CSW: m.CSW, ID: m.ID, Version: m.Version, API: m.API, Name: m.Name, Description: m.Description,
		Author: m.Author, License: m.License, Category: m.Category, Tags: m.Tags, Icon: m.Icon, Size: m.Size,
		KeepAspect: m.KeepAspect, Fields: m.Fields, Defaults: m.Defaults, Data: m.Data, Sample: m.Sample,
		Alert: m.Alert,
	}
}
