# Bob

Bob is a small utility for building projects! It supports building Docker images, binaries, containers and static file bundles.

![bob-the-builder](https://upload.wikimedia.org/wikipedia/en/c/c5/Bob_the_builder.jpg)

## Usage

Each project to be built requires a `build.yml` file:

```yaml
---
name: example

# multiple builds can be configured per program, and the CLI allows only
# building one at a time
builds:
  # a docker image published to docker.io
  - kind: docker-public
    latest: true
    labels:
      - house.jm.timestamp:${TIMESTAMP}
      - house.jm.repository:github.com:/jonmorehouse/workspace
      - house.jm.git_ref:${GIT_REF}

  # a go binary gets published to the artifacts server configured by the
  # builder, in this case it would end up at `artifacts.jm.house/static-srv-server/...`
  - kind: bundle
    latest: true
    dockerfile: Dockerfile.build
```

To build a project, simply run:

```bash
$ bob <project-name>
```

## Installation

The latest version of this tool can be installed

## Build Types

### Bundle

Bundles are groupings of files uploaded to the artifacts server. They use [artifactor](https://github.com/jonmorehouse/artifactor) to create checksums, manifests and signatures before being uploaded.

All bundle files require a `Dockerfile` which exposes a `/build` script. During the build lifecycle, `bob` will run the `/build` script and then upload the contents of the `/output` directory as a bundle.

### docker image

`bob` supports building and pushing `Docker` images to local or remote registries.

All builds are created in a new build context where the contents of any `symlinked` directories or files are copied over.

### oci image

This support is still a WIP. Behind the scenes, [img]() is used to build OCI compliant images, which are uploaded as bundles.
