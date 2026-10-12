# csw

`.csw` is the single-file format CroStream uses to publish custom widgets. A
`.csw` file is one JSON document holding a widget's metadata, settings
schema, code and optional assets (base64). This module reads, writes,
validates and signs them. It uses only the Go standard library.

## The file

Keys appear in this order. Lists and maps are never `null`.

| Key | Meaning |
| --- | --- |
| `csw` | Format version, `1`. |
| `id` | Lowercase dotted words, at most 64 characters, e.g. `crostream.alert-neon-burst`. Official widgets start with `crostream.`. |
| `version` | `MAJOR.MINOR.PATCH`. A published version is immutable. |
| `api` | The platform API level the widget needs (below). |
| `name`, `description` | Display text (name 1 to 60 characters, description up to 500). |
| `author` | `{"name": ..., "url": ...}`; the URL is optional and must be http(s). |
| `license` | An SPDX identifier or `proprietary`. |
| `category`, `tags`, `icon` | Filing and display. |
| `size`, `keepAspect` | The size of a new element in canvas pixels (1 to 16384) and whether resizing keeps the ratio. |
| `fields` | The settings schema: a list of props (`key`, `label`, `kind`, ...). Kinds: `text`, `textarea`, `number`, `bool`, `color`, `enum`, `media`, `font`, `effects`, `frame`, `items`, `folder`. |
| `defaults` | A JSON object of the fields' initial values. |
| `data` | Live data keys or prefixes (`chat`, `latest.*`) the widget reads. |
| `sample` | Preview data by data key. |
| `alert` | Optional: the events the widget styles, with sample variables, a hold time and a sound field. |
| `code` | `{"html": ..., "css": ..., "js": ...}`; the script is optional. |
| `assets` | Optional: files by path, `{"type": media type, "data": base64}`. |

API levels: `1` is the base `sl` runtime, `2` adds `sl.alert`, `sl.leave` and
`sl.sfx`, `3` adds `sl.asset` (required for any widget with assets).
`DetectAPI` finds the lowest level a script needs.

Media type: `application/vnd.crostream.widget+json`.

### Assets

At most 64 assets, 2 MiB each and 8 MiB together. Paths are lowercase letters,
digits, `.`, `_`, `-` and `/` between names (at most 128 characters, no
`..`). Allowed types: `image/png`, `image/webp`, `image/svg+xml`, `image/gif`,
`image/jpeg`, `audio/ogg`, `audio/mpeg`, `audio/wav`, `font/woff2`. The
extension must match the type and the bytes must start like the type.

## Validation

`Package.Validate` checks the identity (id, version, API level, author,
license, category) and then the widget with `ValidateDefinition`: name and text
limits, size, the field schema (keys, kinds, bounds, options, nested lists),
the defaults against the fields, code size (256 KiB each of HTML, CSS and JS),
data keys, sample data, the alert block and the assets. `ValidateDefinition`
also normalizes its argument (trims, fills nil lists and the defaults). Every
failure wraps `csw.ErrInvalid`; a file that is not a `.csw` document wraps
`csw.ErrFormat`.

`Package.Warnings` lists what is allowed but wrong: a declared API level that
differs from what the script needs, and assets the code never mentions.

## Source folders

A widget can be kept as a folder named after its id: `manifest.json` (the
metadata), `widget.html`, `widget.css`, optional `widget.js`, and optional
`assets/`. `FromDir` and `ToDir` convert between a folder and a `Package`.

## Gallery layout

A gallery is a static folder (`dist/`, served as `/v1/`):

```
/v1/index.json                    the catalog of every widget and version
/v1/index.json.sig                base64 Ed25519 signature of index.json's exact bytes
/v1/w/<id>/<version>.csw          one file per published version
```

Each version in the index carries its URL, SHA-256, size, publish time and API
level. Keys are Ed25519: the private key is PKCS#8 PEM, the public key PKIX
PEM.

## Command

```
go run github.com/crostream/csw/cmd/csw@latest <command>

pack <src-root> -o <dist>          validate source folders, write dist/w/<id>/<version>.csw
check <src-root|folder|file.csw>   validate sources or a .csw file
index <dist> -key <private key>    write and sign dist/index.json [-featured id,id] [-now RFC3339]
verify <dist> -pub <public key>    check the signature and every file the index lists
genkey -out <file> [-force]        write a keypair (<file> and <file>.pub, mode 0600 for the private key)
```

Exit codes: 0 success, 1 failure, 2 bad usage. `pack` refuses to change an
already published version.

## Go

```go
p, err := csw.Decode(f)
if err != nil { ... }          // errors.Is(err, csw.ErrFormat)
if err := p.Validate(); err != nil { ... } // errors.Is(err, csw.ErrInvalid)

data, err := csw.Encode(p)     // canonical bytes: equal packages encode equally
```

`ValidateDefinition(*Definition)` validates a widget that is held somewhere
other than a `.csw` file; a `DefinitionAsset` may carry only its `SHA256` and
`Size` when the bytes are stored elsewhere. `BuildIndex`, `WriteIndex`,
`VerifyIndex` and `VerifyDist` work on a gallery folder; `GenerateKey`,
`ParsePrivateKey` and `ParsePublicKey` handle the signing keys.

## License

Apache-2.0; see `LICENSE`.
