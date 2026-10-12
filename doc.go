// Package csw reads, writes and checks .csw files: the single-file format
// CroStream's external widget gallery uses to publish custom widgets. A
// .csw file is one JSON document holding a widget's metadata, schema, code
// and optional assets; see Package.
//
// The rules a widget must meet are in ValidateDefinition (the widget
// itself) and Package.Validate (that, plus the file's identity). The
// package also holds what the gallery's tooling needs around the format:
// the gallery source folder layout (FromDir, ToDir), the signed gallery
// index (Index, Sign, Verify) and the signing keys (GenerateKey,
// ParsePrivateKey, ParsePublicKey). The csw command in cmd/csw wraps them.
//
// Every validation error wraps ErrInvalid, and every error from reading a
// file that is not a .csw document wraps ErrFormat.
package csw
