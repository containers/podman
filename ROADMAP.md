![PODMAN logo](https://raw.githubusercontent.com/containers/common/main/logos/podman-logo-full-vert.png)

# Podman Roadmap

The Podman development team reviews feature requests from its various stakeholders for consideration
quarterly.  Podman maintainers then prioritize these features.   Top features are then assigned to
one or more engineers.


## Future feature considerations

The following features are of general importantance to Podman.  While these features have no timeline
associated with them yet, they will likely be on future quarterly milestones.

* Further improvements to `podman machine` to better support Podman Desktop and other developer usecases.
  - Smoother upgrade process for Podman machine operating system (OS) images
  - Convergence of WSL technologies with other providers including its OS
* Remote client support for OCI artifacts and its RESTFUL API
* Integration of composefs
* Ongoing work around partial pull support (zstd:chunked)
* Improved support for the BuildKit API.
* Performance and stability improvements.
* Reductions to the size of the Podman binary.

## Milestones and commitments by quarter

This section is a historical account of what features were prioritized by quarter.  Results of the prioritization will be added at start of each quarter (Jan, Apr, July, Oct).

### 2025 Q2 ####

#### Releases ####
- [ ] Podman 5.5
- [ ] Fully automated Podman releases

#### Features ####
- [ ] Windows ARM64 installer
- [ ] Add support for artifacts in RESTFUL service
- [ ] Reduce binary size of Podman
- [ ] Add remote client support for artifacts
- [ ] Add support for newer Docker API versions to RESTFUL service
- [ ] Replace Podman pause image with a rootfs

#### CNCF ####
- [ ] Add and adhere to Governance model

### 2025 Q1 ####

#### Releases ####
- [x] Podman 5.4
- [x] Podman release automation

#### Features ####
- [x] Artifact add --append
- [x] Artifact extract
- [x] Artifact add --options
- [x] Mount OCI artifacts inside containers
- [x] Determine strategy for configuration files when remote

#### CNCF ####
- [x] Create Maintainers file
- [x] Create Governance documentation
