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

	// ParallelFileWrites enables parallel file writing during tar extraction.
	// This improves performance on NVMe storage by increasing I/O queue depth.
	// Set to true to enable parallel writes (default: false for backward compatibility)
	ParallelFileWrites bool `toml:"parallel_file_writes"`

	// ParallelFileWriteWorkers is the number of parallel file writer goroutines.
	// Only used when ParallelFileWrites is true. (default: 16)
	ParallelFileWriteWorkers int `toml:"parallel_file_write_workers"`

	// ParallelFileWriteBufferSizeStr is the buffer size for file writes (e.g., "1mb", "512kb").
	// Larger buffers improve throughput on NVMe. (default: 1mb)
	ParallelFileWriteBufferSizeStr string `toml:"parallel_file_write_buffer_size"`
	ParallelFileWriteBufferSize    int    `toml:"-"`
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

	// Parse parallel file write buffer size
	if cfg.PullModes.Parallel.ParallelFileWriteBufferSizeStr != "" {
		bufSize, err := parseSize(cfg.PullModes.Parallel.ParallelFileWriteBufferSizeStr)
		if err != nil {
			return fmt.Errorf("invalid parallel_file_write_buffer_size: %w", err)
		}
		cfg.PullModes.Parallel.ParallelFileWriteBufferSize = int(bufSize)
	} else {
		cfg.PullModes.Parallel.ParallelFileWriteBufferSize = 1 << 20 // 1MB default
	}

	// Set default workers if not specified
	if cfg.PullModes.Parallel.ParallelFileWrites && cfg.PullModes.Parallel.ParallelFileWriteWorkers <= 0 {
		cfg.PullModes.Parallel.ParallelFileWriteWorkers = 16
	}

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
		return 0, fmt.Errorf("failed to parse size number: %w", err)
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
