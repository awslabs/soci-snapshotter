/*
   Copyright The Soci Snapshotter Authors.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package ztoc

import (
	"fmt"

	"github.com/awslabs/soci-snapshotter/ztoc/compression"
)

// Builder holds a single `TocBuilder` that builds toc, and one `ZinfoBuilder`
// *per* compression algorithm that builds zinfo. `TocBuilder` is shared by different
// compression algorithms. Which `ZinfoBuilder` is used depends on the compression
// algorithm used by the layer.
type Builder struct {
	tocBuilder    TocBuilder
	zinfoBuilders map[string]ZinfoBuilder

	buildToolIdentifier string
}

// NewBuilder creates a `Builder` used to build ztocs. By default it supports gzip,
// user can register new compression algorithms by calling `RegisterCompressionAlgorithm`.
func NewBuilder(buildToolIdentifier string) *Builder {
	builder := Builder{
		tocBuilder:          NewTocBuilder(),
		zinfoBuilders:       make(map[string]ZinfoBuilder),
		buildToolIdentifier: buildToolIdentifier,
	}
	builder.RegisterCompressionAlgorithm(compression.Gzip, TarProviderGzip, gzipZinfoBuilder{})
	builder.RegisterCompressionAlgorithm(compression.Uncompressed, TarProviderTar, tarZinfoBuilder{})
	builder.RegisterCompressionAlgorithm(compression.Unknown, TarProviderTar, tarZinfoBuilder{})

	return &builder
}

// buildConfig contains configuration used when `ztoc.Builder` builds a `Ztoc`.
type buildConfig struct {
	algorithm string
}

// BuildOption specifies a change to `buildConfig` when building a ztoc.
type BuildOption func(opt *buildConfig) error

// WithCompression specifies which compression algorithm is used by the layer.
func WithCompression(algorithm string) BuildOption {
	return func(opt *buildConfig) error {
		opt.algorithm = algorithm
		return nil
	}
}

// defaultBuildConfig creates a `buildConfig` with default values.
func defaultBuildConfig() buildConfig {
	return buildConfig{
		algorithm: compression.Gzip, // use gzip by default
	}
}

// BuildZtoc builds a `Ztoc` given the filename of a layer blob. By default it assumes
// the layer is compressed using `gzip`, unless specified via `WithCompression`.
func (b *Builder) BuildZtoc(filename string, span int64, options ...BuildOption) (*Ztoc, error) {
	if filename == "" {
		return nil, fmt.Errorf("need to provide a compressed filename")
	}

	opt := defaultBuildConfig()
	for _, f := range options {
		err := f(&opt)
		if err != nil {
			return nil, err
		}
	}

	if !b.CheckCompressionAlgorithm(opt.algorithm) {
		return nil, fmt.Errorf("unsupported compression algorithm, supported: gzip, got: %s", opt.algorithm)
	}

	compressionInfo, fs, err := b.zinfoBuilders[opt.algorithm].ZinfoFromFile(filename, span)
	if err != nil {
		return nil, err
	}

	toc, uncompressedArchiveSize, err := b.tocBuilder.TocFromFile(opt.algorithm, filename)
	if err != nil {
		return nil, err
	}

	return &Ztoc{
		Version:                 Version09,
		TOC:                     toc,
		CompressedArchiveSize:   fs,
		UncompressedArchiveSize: uncompressedArchiveSize,
		BuildToolIdentifier:     b.buildToolIdentifier,
		CompressionInfo:         compressionInfo,
	}, nil
}

// RegisterCompressionAlgorithm supports a new compression algorithm in `ztoc.Builder`.
func (b *Builder) RegisterCompressionAlgorithm(name string, tarProvider TarProvider, zinfoBuilder ZinfoBuilder) {
	if b.zinfoBuilders == nil {
		b.zinfoBuilders = make(map[string]ZinfoBuilder)
	}
	b.zinfoBuilders[name] = zinfoBuilder
	b.tocBuilder.RegisterTarProvider(name, tarProvider)
}

// CheckCompressionAlgorithm checks if a compression algorithm is supported.
//
// The algorithm has to be supported by both (1) `tocBuilder` (straightforward,
// create a tar reader from the compressed io.reader in compressionFileReader)
// and (2) `zinfoBuilder` (require zinfo impl, see `compression/gzip_zinfo.go` as an example).
func (b *Builder) CheckCompressionAlgorithm(algorithm string) bool {
	_, ok := b.zinfoBuilders[algorithm]
	return ok && b.tocBuilder.CheckCompressionAlgorithm(algorithm)
}
