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

/*
   Copyright The containerd Authors.

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

package metadata

import (
	"io"
	"os"
	"time"

	"github.com/awslabs/soci-snapshotter/compression"
	"github.com/awslabs/soci-snapshotter/ztoc"
)

// Attr reprensents the attributes of a node.
type Attr struct {
	// Size, for regular files, is the logical size of the file.
	Size int64

	// ModTime is the modification time of the node.
	ModTime time.Time

	// LinkName, for symlinks, is the link target.
	LinkName string

	// Mode is the permission and mode bits.
	Mode os.FileMode

	// UID is the user ID of the owner.
	UID int

	// GID is the group ID of the owner.
	GID int

	// DevMajor is the major device number for device.
	DevMajor int

	// DevMinor is the major device number for device.
	DevMinor int

	// Xattrs are the extended attribute for the node.
	Xattrs map[string][]byte

	// NumLink is the number of names pointing to this node.
	NumLink int
}

// Store reads the provided blob and creates a metadata reader.
type Store func(sr *io.SectionReader, ztoc *ztoc.Ztoc, opts ...Option) (Reader, error)

// Reader provides access to file metadata of a blob.
type Reader interface {
	RootID() uint32

	GetAttr(id uint32) (attr Attr, err error)
	GetChild(pid uint32, base string) (id uint32, attr Attr, err error)
	ForeachChild(id uint32, f func(name string, id uint32, mode os.FileMode) bool) error
	OpenFile(id uint32) (File, error)

	Clone(sr *io.SectionReader) (Reader, error)
	Close() error
}

type File interface {
	GetUncompressedFileSize() compression.Offset
	GetUncompressedOffset() compression.Offset
}

type Options struct {
	Telemetry *Telemetry
}

// Option is an option to configure the behaviour of reader.
type Option func(o *Options) error

// WithTelemetry option specifies the telemetry hooks
func WithTelemetry(telemetry *Telemetry) Option {
	return func(o *Options) error {
		o.Telemetry = telemetry
		return nil
	}
}

// A func which takes start time and records the diff
type MeasureLatencyHook func(time.Time)

// A struct which defines telemetry hooks. By implementing these hooks you should be able to record
// the latency metrics of the respective steps of SOCI open operation.
type Telemetry struct {
	InitMetadataStoreLatency MeasureLatencyHook // measure time to initialize metadata store (in milliseconds)
}
