# Releases

## Versioning

Versions follow the `Major.Minor.Patch-PreRelease` scheme of [Semantic Versioning](https://semver.org).

### SOCI CLI / snapshotter / library

The SOCI CLI, snapshotter, and library all share the same version to make it easy to understand compatibility. Collectively, this will be referred to as the "SOCI version".

### zTOC data format

zTOC has its own version which will evolve independently from the other components of SOCI. A bump in zTOC version will translate to a bump in the SOCI version, but the reverse is not true. Since zTOCs are stored in content-addressable registries, changes to the zTOC format will be strongly discouraged to avoid churn for SOCI users.

*__Note:__ The zTOC data format is separately versioned, but the library that's used to build, interact with, and manipulate zTOCs is part of the SOCI library and follows the SOCI version.*

## Supported Versions

The SOCI projct is still under rapid development. As such, official SOCI project support will be in the form of new Major and Minor versions. We may release Patch versions in an *ad hoc* manner if there is sufficient demand from the community.

This policy will be changed once SOCI reaches v1.0.0.

## Releases

Releases are made through [GitHub Releases](https://github.com/awslabs/soci-snapshotter/releases). If there is demand, we will consider creating official linux packages for various distributions.


### Major Version Releases (branch: `main`)

Major versions are developed on the main branch. 

When the time comes to release a new major version, a new `release/Major.0` branch will be created following the same process used for minor releases. 

### Minor Version Releases (branch: `main`)

Minor versions are developed on the main branch. 

When the time comes to release a new minor version, a new `release/Major.Minor` branch will be created from the tip of main. Once the new branch has been created, a new `vMajor.Minor.0` tag will be created following the process used for patch releases.

### Patch Version Releases (branch: `release/Major.Minor`)

Patch versions are developed after the initial minor version tag in the `release/Major.Minor` branch to which they belong. Patch releases are used for security fixes and major or widespread bug fixes. Patch releases will not include new features. 

When the time comes to release a new patch version, a new commit will be added to the `release/Major.Minor` branch to add any necessary release artifacts (e.g. a finalized copy of third party licenses, finalized change log, etc), and a new `vMajor.Minor.Patch` tag will be created from that commit. 

Once the tag is created, a release will automatically be added to github with the same `vMajor.Minor.Patch` version scheme containing the release artifacts. For a full list of artifacts contained in the release, see [Release Artifacts](#release-artifacts).

*__NOTE:__ release automation is currently aspirational. See https://github.com/awslabs/soci-snapshotter/issues/447*

## API Stability

Semantic Versioning expects minor versions to contain only backwards-compatible, new features. In order to avoid partial features from preventing new minor version releases, we do not guarantee that new features are stable when they are introduced. Instead, we maintain the following table indicating the stability of each new feature along with which SOCI version stabilized the feature.

| Feature/API                           | Stability | When it became stable         |
| ------------------------------------- | --------- | ----------------------------- |
| [zTOC data format](#ztoc-data-format) | Beta      | targetting before SOCI v1.0.0 |
| [zTOC API](#ztoc-api)                 | Unstable  | targetting v1.0.0             |
| [SOCI API](#soci-api)                 | Unstable  | targetting v1.0.0             |
| [SOCI CLI](#soci-cli)                 | Unstable  |                               |

### zTOC data format

The zTOC data format is the serialized form of a zTOC. Since zTOCs will be stored in registries and registries are content addressable, we don't anticipate many changes to the zTOC format. 

### zTOC API

The zTOC API is the portion of the SOCI library used for creating and working with zTOCs. 

### SOCI API

The SOCI API is the portion of the SOCI library used for creating and interacting with SOCI indices. 

### SOCI CLI

The SOCI CLI (`bin/soci`) is the binary that can be used to create and inspect SOCI indicies and zTOCs. 


## Release Artifacts

```
Changelog
soci-snapshotter-$VERSION-linux-amd64.tar.gz
soci-snapshotter-$VERSION-linux-amd64.tar.gz.sha256sum
soci-snapshotter-$VERSION-linux-amd64-static.tar.gz
soci-snapshotter-$VERSION-linux-amd64-static.tar.gz.sha256sum
soci-snapshotter-$VERSION-linux-arm64.tar.gz
soci-snapshotter-$VERSION-linux-arm64.tar.gz.sha256sum
soci-snapshotter-$VERSION-linux-arm64-static.tar.gz
soci-snapshotter-$VERSION-linux-arm64-static.tar.gz.sha256sum
Source code (zip)
Source code (tar.gz)
```

Each release tarball contains the following: 
```
soci-snapshotter-grpc
soci
THIRD_PARTY_LICENSES
NOTICE.md
```
 

## Next Release / Release Cadence

The next release is tracked via [GitHub milestones](https://github.com/awslabs/soci-snapshotter/milestones).

The SOCI project doesn’t follow any fixed release cadence. 

