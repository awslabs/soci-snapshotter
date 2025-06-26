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

// DecompressStream specifies the configuration for a decompression implementation.
type DecompressStream struct {
	// Path is the system path to the decompression binary.
	Path string `toml:"path"`

	// Args is a list of command arguments passed to the decompression binary.
	Args []string `toml:"args"`
}

// ParallelConfig modifies behavior for eager image pulls.
// Set any of the TOML vals to negative to unbound any of these operations.
type ParallelConfig struct {
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

func defaultParallelConfig() ParallelConfig {
	return ParallelConfig{
		MaxConcurrentDownloads:         defaultMaxConcurrentDownloads,
		MaxConcurrentDownloadsPerImage: defaultMaxConcurrentDownloadsPerImage,
		ConcurrentDownloadChunkSize:    defaultConcurrentDownloadChunkSize,
		MaxConcurrentUnpacks:           defaultMaxConcurrentUnpacks,
		MaxConcurrentUnpacksPerImage:   defaultMaxConcurrentUnpacksPerImage,
	}
}

func parseParallelConfig(cfg *Config) error {
	size, err := parseSize(cfg.PullModes.Parallel.ConcurrentDownloadChunkSizeStr)
	if err != nil {
		return err
	}
	cfg.PullModes.Parallel.ConcurrentDownloadChunkSize = size
	return nil
}

func parseSize(sizeStr string) (int64, error) {
	if sizeStr == "" {
		return defaultConcurrentDownloadChunkSize, nil // use default value for empty string
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
		size = defaultConcurrentDownloadChunkSize // use default value for zero size
	}

	return size, nil
}
