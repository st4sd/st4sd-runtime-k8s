#!/usr/bin/env bash

# Copyright IBM Inc. All Rights Reserved.
# SPDX-License-Identifier: Apache-2.0
# Authors:
#   Vassilis Vassiliadis


# VV: Expects IMAGE_BASE_URL, DOCKER_REGISTRY, DOCKER_TOKEN, DOCKER_USERNAME,
#   LABEL, SRC_TAG_X8664, SRC_TAG_PPC64LE, DST_TAG_X8664, DST_TAG_PPC64LE

set -euxo pipefail -o xtrace

export DOCKER_CLI_EXPERIMENTAL=enabled
BASEDIR=$(dirname "$0")
cd ${BASEDIR}

docker login -u $DOCKER_USERNAME -p $DOCKER_TOKEN $DOCKER_REGISTRY

docker pull quay.io/skopeo/stable

echo "Copying images"
docker run --rm -it \
      --env DOCKER_REGISTRY --env DOCKER_TOKEN --env DOCKER_USERNAME \
      -v `pwd`:/scripts -w /scripts --entrypoint /scripts/skopeo_copy.sh quay.io/skopeo/stable \
      ${IMAGE_BASE_URL}:${SRC_TAG_X8664} ${IMAGE_BASE_URL}:${DST_TAG_X8664}

docker run --rm -it \
      --env DOCKER_REGISTRY --env DOCKER_TOKEN --env DOCKER_USERNAME \
      -v `pwd`:/scripts -w /scripts --entrypoint /scripts/skopeo_copy.sh quay.io/skopeo/stable \
      ${IMAGE_BASE_URL}:${SRC_TAG_PPC64LE} ${IMAGE_BASE_URL}:${DST_TAG_PPC64LE}

echo "Creating multi-arch manifest"

docker manifest create ${IMAGE_BASE_URL}:${LABEL} \
      ${IMAGE_BASE_URL}:${DST_TAG_X8664} ${IMAGE_BASE_URL}:${DST_TAG_PPC64LE}

echo "Annotating architectures of images in manifest"
docker manifest annotate --arch=amd64 ${IMAGE_BASE_URL}:${LABEL} ${IMAGE_BASE_URL}:${DST_TAG_X8664}

docker manifest annotate --arch=ppc64le ${IMAGE_BASE_URL}:${LABEL} ${IMAGE_BASE_URL}:${DST_TAG_PPC64LE}

echo "Pushing manifest"
docker manifest push ${IMAGE_BASE_URL}:${LABEL}
