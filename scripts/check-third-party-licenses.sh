#!/usr/bin/env bash

# Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License"). You
# may not use this file except in compliance with the License. A copy of
# the License is located at
#
# 	http://aws.amazon.com/apache2.0/
#
# or in the "license" file accompanying this file. This file is
# distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
# ANY KIND, either express or implied. See the License for the specific
# language governing permissions and limitations under the License.

set -euo pipefail

# Normalize to working directory being root (up one level from ./scripts)
root=$( cd "$( dirname "${BASH_SOURCE[0]}" )/.." && pwd )

pushd "${root}"

# Fail third party dependency usage if not covered by the curated set of pre-approved licenses.
#
# List was generated from guidance set forth by Amazon open source usage policies.
#
# The SOCI project, with the exception of its usage of MPL-2.0, additionally follows the guidance
# set forth by the CNCF Allowlist License Policy.
#
# https://github.com/cncf/foundation/blob/88f1a47550eb2df71b4b6e9c148a1c2f99a1d92e/allowed-third-party-license-policy.md
#
# Additional usage of third party dependencies not covered by the following licenses
# will need maintainer approval in alignment with Amazon open source usage policies.
#
# Requests can be made via https://github.com/awslabs/soci-snapshotter/issues/new/choose
go-licenses check \
    --include_tests \
    --ignore github.com/awslabs/soci-snapshotter \
    --allowed_licenses=Apache-2.0,BSD-2-Clause,BSD-2-Clause-FreeBSD,BSD-3-Clause,MIT,ISC,Python-2.0,PostgreSQL,X11,Zlib,MPL-2.0 ./...

popd
