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
	"errors"
	"fmt"

	"github.com/awslabs/soci-snapshotter/soci"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	artifactsspec "github.com/oras-project/artifacts-spec/specs-go/v1"
	"oras.land/oras-go/v2/content"
)

var (
	ErrNoReferrers = errors.New("no existing referrers")
)

// Determines which index will be selected from a list of index descriptors
type IndexSelectionPolicy func([]ocispec.Descriptor) (ocispec.Descriptor, error)

func SelectFirstPolicy(descs []ocispec.Descriptor) (ocispec.Descriptor, error) {
	return descs[0], nil
}

// Responsible for making Referrers API calls to remote registry to fetch list of referrers.
type ReferrersClient interface {
	/// Takes in an manifest descriptor and IndexSelectionPolicy and returns a single artifact descriptor.
	/// Returns an error (ErrNoReferrers) if the manifest descriptor has no referrers.
	SelectReferrer(context.Context, ocispec.Descriptor, IndexSelectionPolicy) (ocispec.Descriptor, error)
}

// Interface for oras-go's Repository.Referrers call, for mocking
type ORASReferrersCaller interface {
	Referrers(ctx context.Context, desc ocispec.Descriptor, artifactType string, fn func(referrers []artifactsspec.Descriptor) error) error
}

type Inner interface {
	content.Storage
	ORASReferrersCaller
}

type OrasClient struct {
	Inner
}

func NewORASClient(inner Inner) *OrasClient {
	return &OrasClient{
		Inner: inner,
	}
}

func (c *OrasClient) SelectReferrer(ctx context.Context, desc ocispec.Descriptor, fn IndexSelectionPolicy) (ocispec.Descriptor, error) {
	descs, err := c.allReferrers(ctx, desc)
	if err != nil {
		return ocispec.Descriptor{}, fmt.Errorf("unable to fetch referrers: %w", err)
	}
	if len(descs) == 0 {
		return ocispec.Descriptor{}, ErrNoReferrers
	}
	return fn(descs)
}

func (c *OrasClient) allReferrers(ctx context.Context, desc ocispec.Descriptor) ([]ocispec.Descriptor, error) {
	descs := []ocispec.Descriptor{}
	err := c.Referrers(ctx, desc, soci.SociIndexArtifactType, func(referrers []artifactsspec.Descriptor) error {
		for _, v := range referrers {
			descs = append(descs, artifactToOCIDesc(v))
		}
		return nil
	})
	return descs, err
}

func artifactToOCIDesc(ar artifactsspec.Descriptor) ocispec.Descriptor {
	return ocispec.Descriptor{
		MediaType:   ar.MediaType,
		Digest:      ar.Digest,
		Size:        ar.Size,
		Annotations: ar.Annotations,
		URLs:        ar.URLs,
	}
}
