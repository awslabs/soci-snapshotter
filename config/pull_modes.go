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

package config

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// PullModes contain config related to the ways in
// in which the SOCI snapshotter can pull images
type PullModes struct {
	SOCIv1             V1                 `toml:"soci_v1"`
	SOCIv2             V2                 `toml:"soci_v2"`
	ParallelPullUnpack ParallelPullUnpack `toml:"parallel_pull_unpack"`
}

// V1 contains config for SOCI v1 which uses the
// OCI referrers API to automatically discover SOCI
// indexes that reference an image
type V1 struct {
	Enable bool `toml:"enable"`
}

// V2 contains config for SOCI v2 which uses annotations
// on the container's image manifest to discover SOCI indexes
// without an out-of-band referrers API call
type V2 struct {
	Enable bool `toml:"enable"`
}

// ParallelPull modifies behavior for eager image pulls.
// Set any of the TOML vals to negative to unbound any of these operations.
type ParallelPullUnpack struct {
	Enable bool `toml:"enable"`

	MaxConcurrentDownloads         int64 `toml:"max_concurrent_downloads"`
	MaxConcurrentDownloadsPerImage int64 `toml:"max_concurrent_downloads_per_image"`

	ConcurrentDownloadChunkSizeStr string `toml:"concurrent_download_chunk_size"`
	ConcurrentDownloadChunkSize    int64  `toml:"-"`

	MaxConcurrentUnpacks         int64 `toml:"max_concurrent_unpacks"`
	MaxConcurrentUnpacksPerImage int64 `toml:"max_concurrent_unpacks_per_image"`

	// DecompressStreams modifies the implementations used to unpack compressed layer tarballs.
	DecompressStreams map[string]DecompressStream `toml:"decompress_streams"`

	DiscardUnpackedLayers bool `toml:"discard_unpacked_layers"`
}

// DecompressStream specifies the configuration for a decompression implementation.
type DecompressStream struct {
	// Path is the system path to the decompression binary.
	Path string `toml:"path"`

	// Args is a list of command arguments passed to the decompression binary.
	Args []string `toml:"args"`
}

func defaultPullModes(cfg *Config) error {
	cfg.PullModes = DefaultPullModes()
	return nil
}

// DefaultPullModes returns a PullModes struct
// with the SOCI defaults set.
func DefaultPullModes() PullModes {
	return PullModes{
		SOCIv1: V1{
			Enable: DefaultSOCIV1Enable,
		},
		SOCIv2: V2{
			Enable: DefaultSOCIV2Enable,
		},
		ParallelPullUnpack: ParallelPullUnpack{
			Enable:                         DefaultParallelPullUnpackEnable,
			MaxConcurrentDownloads:         defaultMaxConcurrentDownloads,
			MaxConcurrentDownloadsPerImage: defaultMaxConcurrentDownloadsPerImage,
			ConcurrentDownloadChunkSize:    defaultConcurrentDownloadChunkSize,
			MaxConcurrentUnpacks:           defaultMaxConcurrentUnpacks,
			MaxConcurrentUnpacksPerImage:   defaultMaxConcurrentUnpacksPerImage,
		},
	}
}

var (
	sizeRegex = regexp.MustCompile(`(?i)^\s*(\d+(\.\d+)?)(\s*(gb|mb|kb|b)?)?\s*$`)

	unitMultipliers = map[string]float64{
		"":   1, // no unit specified, treat as bytes
		"b":  1,
		"kb": 1024,
		"mb": 1024 * 1024,
		"gb": 1024 * 1024 * 1024,
	}
)

func parseImagePullConfig(cfg *Config) error {
	sizeStr := cfg.PullModes.ParallelPullUnpack.ConcurrentDownloadChunkSizeStr
	if sizeStr != "" {
		size, err := parseSize(sizeStr)
		if err != nil {
			return err
		}
		cfg.PullModes.ParallelPullUnpack.ConcurrentDownloadChunkSize = size
	}
	return nil
}

func parseSize(sizeStr string) (int64, error) {
	if sizeStr == "" {
		return -1, nil // use default value for empty string
	}

	matches := sizeRegex.FindStringSubmatch(sizeStr)
	if matches == nil {
		return 0, fmt.Errorf("invalid size format: %s", sizeStr)
	}

	numStr, unitStr := matches[1], strings.ToLower(strings.TrimSpace(matches[4]))
	num, err := strconv.ParseFloat(numStr, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse size number: %v", err)
	}

	multiplier, ok := unitMultipliers[unitStr]
	if !ok {
		return 0, fmt.Errorf("unknown size unit: %s", unitStr)
	}

	size := int64(num * multiplier)
	if size < 0 {
		return 0, fmt.Errorf("size cannot be negative: %s", sizeStr)
	}
	if size == 0 {
		size = -1 // use default value for zero size
	}

	return size, nil
}
