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

package fs

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/awslabs/soci-snapshotter/config"
	"github.com/google/go-cmp/cmp"
	"go.uber.org/goleak"
)

const (
	testGarbageCollectionFrequency = 100 * time.Millisecond
)

func init() {
	garbageCollectionInterval = testGarbageCollectionFrequency
}

var (
	helloWorldImageDigest = "sha256:7e1a4e2d11e2ac7a8c3f768d4166c2defeb09d2a750b010412b6ea13de1efb19"
	helloWorldLayerDigest = "sha256:e6590344b1a5dc518829d6ea1524fc12f8bcd14ee9a02aa6ad8360cce3a9a9e9"
)

func newEnableParallelPullConfig() *config.Parallel {
	return &config.Parallel{
		Enable: true,
	}
}

func TestParallelPullUnpackValidation(t *testing.T) {
	tc := []struct {
		name       string
		cfg        *config.Parallel
		expectFail bool
	}{
		{
			name: "empty image pull config is valid",
			cfg:  newEnableParallelPullConfig(),
		},
		{
			name: "logical image pull config is valid",
			cfg: &config.Parallel{
				Enable: true,
				ParallelConfig: config.ParallelConfig{
					MaxConcurrentDownloads:         9,
					MaxConcurrentDownloadsPerImage: 3,
					MaxConcurrentUnpacks:           3,
					MaxConcurrentUnpacksPerImage:   1,
				},
			},
		},
		{
			name: "illogical image pull config is not valid",
			cfg: &config.Parallel{
				Enable: true,
				ParallelConfig: config.ParallelConfig{
					MaxConcurrentDownloads:         1,
					MaxConcurrentDownloadsPerImage: 3,
					MaxConcurrentUnpacks:           1,
					MaxConcurrentUnpacksPerImage:   3,
				},
			},
			expectFail: true,
		},
	}
	for _, tt := range tc {
		t.Run(tt.name, func(t *testing.T) {
			testCtx, cancel := context.WithCancel(context.Background())
			defer cancel()

			_, err := newUnpackJobs(testCtx, tt.cfg, newVirtualDisk())
			if (err != nil) != tt.expectFail {
				if tt.expectFail {
					t.Fatalf("expected bad image config to fail but succeeded")
				} else {
					t.Fatalf("expected good image config to succeed but failed")
				}
			}
		})
	}
}

// TestNoGoroutinesAreLeakedWhenGarbageCollectionIsCancelled asserts that garbage collection
// goroutines are cleaned up when the context is cancelled.
func TestNoGoroutinesAreLeakedWhenGarbageCollectionIsCancelled(t *testing.T) {
	defer goleak.VerifyNone(t)

	testCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	_, _ = newUnpackJobs(testCtx, newEnableParallelPullConfig(), newVirtualDisk())
	await(3 * ticks)
}

// TestSystemResourcesAreGarbageCollectedForCompletedJobs asserts all completed image unpack jobs
// are cleaned up by the garbage collector.
func TestSystemResourcesAreGarbageCollectedForCompletedJobs(t *testing.T) {
	defer goleak.VerifyNone(t)

	testCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	disk := newVirtualDisk()
	disk.CreateCompletedImageUnpackJobs(3 * jobs)

	inProgressJobs, _ := newUnpackJobs(testCtx, newEnableParallelPullConfig(), disk)
	await(3 * ticks)

	disk.AssertAllUnusedResourcesHaveBeenGarbageCollected(t, inProgressJobs)
}

// TestInProgressJobsAreNotGarbageCollected asserts any in-progress image unpack jobs
// are not marked and reclaimed by the garbage collector.
func TestInProgressJobsAreNotGarbageCollected(t *testing.T) {
	defer goleak.VerifyNone(t)

	testCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	disk := newVirtualDisk()
	disk.CreateCompletedImageUnpackJobs(3 * jobs)

	inProgressJobs, _ := newUnpackJobs(testCtx, newEnableParallelPullConfig(), disk)
	createEphemeralInProgressImageUnpackJob(t, inProgressJobs)
	await(3 * ticks)

	disk.AssertAllUsedResourcesHaveNotBeenGarbageCollected(t, inProgressJobs)
}

// TestExpiredJobsAreGarbageCollected asserts any in-progress image unpack jobs
// that have been running longer than the default expiry time are cancelled and reclaimed
// by the garbage collector.
func TestExpiredJobsAreGarbageCollected(t *testing.T) {
	defer goleak.VerifyNone(t)

	testCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	disk := newVirtualDisk()
	inProgressJobs, _ := newUnpackJobs(testCtx, newEnableParallelPullConfig(), disk)
	createExpiredInProgressImageUnpackJob(t, inProgressJobs)

	await(3 * ticks)

	disk.AssertAllUnusedResourcesHaveBeenGarbageCollected(t, inProgressJobs)
}

const (
	ticks = 1
	jobs  = 1
)

func await(numberOfTicks int) {
	ticker := time.NewTicker(garbageCollectionInterval)
	defer ticker.Stop()

	for range ticker.C {
		if numberOfTicks == 0 {
			return
		}
		numberOfTicks--
	}
}

func createEphemeralInProgressImageUnpackJob(t testing.TB, inProgressJobs *unpackJobs) {
	imageJob := inProgressJobs.GetOrAddImageJob(helloWorldImageDigest, func(cause error) {})
	_, err := inProgressJobs.AddLayerJob(imageJob, helloWorldLayerDigest)
	if err != nil {
		t.Fatalf("Failed to create ephemeral in-progress image unpack job: %v", err)
	}
}

func createExpiredInProgressImageUnpackJob(t testing.TB, inProgressJobs *unpackJobs) {
	originalNow := now
	defer func() {
		now = originalNow
	}()

	// Set the current time to be in the past so the created job is expired.
	now = func() time.Time {
		return time.Now().Add(-1 * garbageCollectionJobExpiration)
	}

	imageJob := inProgressJobs.GetOrAddImageJob(helloWorldImageDigest, func(cause error) {})
	_, err := inProgressJobs.AddLayerJob(imageJob, helloWorldLayerDigest)
	if err != nil {
		t.Fatalf("Failed to create ephemeral in-progress image unpack job: %v", err)
	}
}

type LayerUnpackVirtualStorage struct {
	jobs sync.Map
}

func newVirtualDisk() *LayerUnpackVirtualStorage {
	return &LayerUnpackVirtualStorage{}
}

func (virtual *LayerUnpackVirtualStorage) Create() (string, error) {
	id, err := virtual.generateUniqueKey()
	if err != nil {
		return "", err
	}

	virtual.jobs.Store(id, struct{}{})

	return id, nil
}

func (virtual *LayerUnpackVirtualStorage) generateUniqueKey() (string, error) {
	for range 10 {
		id := generateUniqueString(defaultUnpackDirLen)
		if _, ok := virtual.jobs.Load(id); !ok {
			return id, nil
		}
	}
	return "", errUniqueJobIDGenFailure
}

func (virtual *LayerUnpackVirtualStorage) GetJobPath(id string) (string, error) {
	if _, ok := virtual.jobs.Load(id); ok {
		return id, nil
	}
	return "", errJobNotFound
}

func (virtual *LayerUnpackVirtualStorage) Keys() ([]string, error) {
	jobs := []string{}

	virtual.jobs.Range(func(key, value any) bool {
		jobs = append(jobs, key.(string))
		return true
	})

	return jobs, nil
}

func (virtual *LayerUnpackVirtualStorage) Delete(id string) error {
	if _, ok := virtual.jobs.Load(id); !ok {
		return errJobNotFound
	}

	virtual.jobs.Delete(id)
	return nil
}

func (virtual *LayerUnpackVirtualStorage) CreateCompletedImageUnpackJobs(numberOfJobs int) {
	for range numberOfJobs {
		_, _ = virtual.Create()
	}
}

func (virtual *LayerUnpackVirtualStorage) AssertAllUnusedResourcesHaveBeenGarbageCollected(t testing.TB, inProgressJobs *unpackJobs) {
	snapshot, err := inProgressJobs.Snapshot(context.Background())
	if err != nil {
		t.Fatalf("Failed to snapshot state: %v", err)
	}

	virtual.jobs.Range(func(key, value any) bool {
		if _, ok := snapshot.inMemory[key.(string)]; ok {
			t.Fatalf("Expected all unused resources to be garbage collected, but found %s was not reclaimed", key)
		}
		return true
	})
}

func (virtual *LayerUnpackVirtualStorage) AssertAllUsedResourcesHaveNotBeenGarbageCollected(t testing.TB, inProgressJobs *unpackJobs) {
	snapshot, err := inProgressJobs.Snapshot(context.Background())
	if err != nil {
		t.Fatalf("Failed to snapshot state: %v", err)
	}

	for id := range snapshot.inMemory {
		if _, ok := virtual.jobs.Load(id); !ok {
			t.Fatalf("Expected all used resources to be not garbage collected, but did not find %s in persistent storage", id)
		}
	}
}

func TestLayerUnpackDiskStorage(t *testing.T) {
	type testContextKey string

	subtests := []struct {
		name    string
		arrange func(testing.TB) (context.Context, LayerUnpackJobStorage)
		act     func(testing.TB, context.Context, LayerUnpackJobStorage) context.Context
		assert  func(testing.TB, context.Context, LayerUnpackJobStorage)
	}{
		{
			name: "CreateEmptyUnpackDirectoryIfItAlreadyExists",
			arrange: func(t testing.TB) (context.Context, LayerUnpackJobStorage) {
				root := t.TempDir()
				rootUnpack := filepath.Join(root, unpackDir)
				if err := os.MkdirAll(rootUnpack, unpackDirPerm); err != nil {
					t.Fatalf("Failed to create directory: %v", err)
				}

				markerPath := filepath.Join(rootUnpack, "marker")
				if _, err := os.Create(markerPath); err != nil {
					t.Fatalf("Failed to create marker file: %v", err)
				}

				uut, err := newLayerUnpackDiskStorage(root)
				if err != nil {
					t.Fatalf("Failed to create uut: %v", err)
				}
				return context.WithValue(context.Background(), testContextKey("marker"), markerPath), uut
			},
			act: func(_ testing.TB, ctx context.Context, _ LayerUnpackJobStorage) context.Context {
				return ctx
			},
			assert: func(t testing.TB, ctx context.Context, _ LayerUnpackJobStorage) {
				markerPath := ctx.Value(testContextKey("marker")).(string)
				if _, err := os.Stat(markerPath); !os.IsNotExist(err) {
					t.Fatalf("Expected marker file to be deleted, but it still exists")
				}
			},
		},
		{
			name: "Create",
			arrange: func(t testing.TB) (context.Context, LayerUnpackJobStorage) {
				uut, err := newLayerUnpackDiskStorage(t.TempDir())
				if err != nil {
					t.Fatalf("Failed to create uut: %v", err)
				}
				return context.Background(), uut
			},
			act: func(t testing.TB, ctx context.Context, uut LayerUnpackJobStorage) context.Context {
				id, err := uut.Create()
				if err != nil {
					t.Fatalf("Failed to create job: %v", err)
				}
				return context.WithValue(ctx, testContextKey("id"), id)
			},
			assert: func(t testing.TB, ctx context.Context, uut LayerUnpackJobStorage) {
				id := ctx.Value(testContextKey("id")).(string)
				if _, err := uut.GetJobPath(id); err != nil {
					t.Fatalf("Failed to fetch job: %v", err)
				}
			},
		},
		{
			name: "Keys",
			arrange: func(t testing.TB) (context.Context, LayerUnpackJobStorage) {
				uut, err := newLayerUnpackDiskStorage(t.TempDir())
				if err != nil {
					t.Fatalf("Failed to create uut: %v", err)
				}

				ids := []string{}
				for range 3 {
					id, err := uut.Create()
					if err != nil {
						t.Fatalf("Failed to create job: %v", err)
					}
					ids = append(ids, id)
				}

				return context.WithValue(context.Background(), testContextKey("expectedIDs"), ids), uut
			},
			act: func(t testing.TB, ctx context.Context, uut LayerUnpackJobStorage) context.Context {
				ids, err := uut.Keys()
				if err != nil {
					t.Fatalf("Failed to list all jobs: %v", err)
				}
				return context.WithValue(ctx, testContextKey("actualIDs"), ids)
			},
			assert: func(t testing.TB, ctx context.Context, uut LayerUnpackJobStorage) {
				expectedIDs := ctx.Value(testContextKey("expectedIDs")).([]string)
				actualIDs := ctx.Value(testContextKey("actualIDs")).([]string)
				if len(expectedIDs) != len(actualIDs) {
					t.Fatalf("Expected %d jobs, but got %d jobs", len(expectedIDs), len(actualIDs))
				}

				slices.Sort(expectedIDs)
				slices.Sort(actualIDs)

				if cmp.Diff(expectedIDs, actualIDs) != "" {
					t.Fatalf("Expected %v, but got %v", expectedIDs, actualIDs)
				}
			},
		},
		{
			name: "GetJobPath",
			arrange: func(t testing.TB) (context.Context, LayerUnpackJobStorage) {
				root := t.TempDir()
				uut, err := newLayerUnpackDiskStorage(root)
				if err != nil {
					t.Fatalf("Failed to create uut: %v", err)
				}
				unpackDir := filepath.Join(root, unpackDir)
				return context.WithValue(context.Background(), testContextKey("root"), unpackDir), uut
			},
			act: func(t testing.TB, ctx context.Context, uut LayerUnpackJobStorage) context.Context {
				id, err := uut.Create()
				if err != nil {
					t.Fatalf("Failed to create job: %v", err)
				}
				return context.WithValue(ctx, testContextKey("id"), id)
			},
			assert: func(t testing.TB, ctx context.Context, uut LayerUnpackJobStorage) {
				root := ctx.Value(testContextKey("root")).(string)
				id := ctx.Value(testContextKey("id")).(string)

				path, err := uut.GetJobPath(id)
				if err != nil {
					t.Fatalf("Failed to fetch job: %v", err)
				}

				if path != filepath.Join(root, id) {
					t.Fatalf("Expected %s, but got %s", filepath.Join(root, id), path)
				}
			},
		},
		{
			name: "Delete",
			arrange: func(t testing.TB) (context.Context, LayerUnpackJobStorage) {
				uut, err := newLayerUnpackDiskStorage(t.TempDir())
				if err != nil {
					t.Fatalf("Failed to create uut: %v", err)
				}
				id, err := uut.Create()
				if err != nil {
					t.Fatalf("Failed to create job: %v", err)
				}
				return context.WithValue(context.Background(), testContextKey("id"), id), uut
			},
			act: func(t testing.TB, ctx context.Context, uut LayerUnpackJobStorage) context.Context {
				err := uut.Delete(ctx.Value(testContextKey("id")).(string))
				if err != nil {
					t.Fatalf("Failed to delete job: %v", err)
				}
				return ctx
			},
			assert: func(t testing.TB, ctx context.Context, uut LayerUnpackJobStorage) {
				id := ctx.Value(testContextKey("id")).(string)
				keys, err := uut.Keys()
				if err != nil {
					t.Fatalf("Failed to list all jobs: %v", err)
				}
				if slices.Contains(keys, id) {
					t.Fatalf("Expected job to be deleted, but found it")
				}
			},
		},
	}

	for _, test := range subtests {
		t.Run(test.name, func(t *testing.T) {
			testCtx, uut := test.arrange(t)
			testCtx = test.act(t, testCtx, uut)
			test.assert(t, testCtx, uut)
		})
	}
}

func TestLayerUnpackJob(t *testing.T) {
	defer goleak.VerifyNone(t)

	testCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	disk := newVirtualDisk()
	inProgressJobs, err := newUnpackJobs(testCtx, newEnableParallelPullConfig(), disk)
	if err != nil {
		t.Fatalf("Expected no setup error, got %v", err)
	}
	createEphemeralInProgressImageUnpackJob(t, inProgressJobs)

	ephemeralJob, err := inProgressJobs.Claim(helloWorldImageDigest, helloWorldLayerDigest)
	if err != nil {
		t.Fatalf("Failed to claim ephemeral job: %v", err)
	}
	if ephemeralJob == nil {
		t.Fatalf("Expected to claim ephemeral job, but did not")
	}
	if status := ephemeralJob.status.Load(); status != LayerUnpackJobClaimed {
		t.Fatalf("Expected job to be claimed, but was %v", status)
	}

	path, err := disk.GetJobPath(ephemeralJob.layerUnpackID)
	if err != nil {
		t.Fatalf("Failed to fetch job path: %v", err)
	}

	expectedIngestPath := filepath.Join(path, helloWorldLayerDigest)
	ingestPath := ephemeralJob.GetIngestLocation()
	if ingestPath != expectedIngestPath {
		t.Fatalf("Expected ingest path to be %s, but got %s", expectedIngestPath, ingestPath)
	}

	expectedUnpackUpperPath := filepath.Join(path, "fs")
	upperPath := ephemeralJob.GetUnpackUpperPath()
	if upperPath != expectedUnpackUpperPath {
		t.Fatalf("Expected unpack upper path to be %s, but got %s", expectedUnpackUpperPath, upperPath)
	}
}

func TestParallelStructCreation(t *testing.T) {
	defer goleak.VerifyNone(t)

	disk := newVirtualDisk()
	testCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tests := []struct {
		name      string
		cfg       *config.Parallel
		expectNil bool
	}{
		{
			name: "with parallel pull enabled",
			cfg: &config.Parallel{
				Enable: true,
			},
		},
		{
			name: "with parallel pull disabled",
			cfg: &config.Parallel{
				Enable: false,
			},
			expectNil: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			unpackJobs, err := createParallelPullStructs(testCtx, disk, tc.cfg)
			if err != nil {
				t.Fatalf("unexpected error creating pull struct: %v", err)
			}
			if (unpackJobs == nil) != tc.expectNil {
				t.Fatalf("expected unpackJobs == nil to be %t, but got %t", tc.expectNil, unpackJobs == nil)
			}
		})
	}

}
