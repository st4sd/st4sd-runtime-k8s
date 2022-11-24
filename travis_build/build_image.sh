#!/usr/bin/env bash
#
# Copyright IBM Inc. All Rights Reserved.
# SPDX-License-Identifier: Apache-2.0
# Authors:
#  Vassilis Vassiliadis


set -euxo pipefail -o xtrace

export DO_PUSH=${DO_PUSH:-yes}
export LABEL=${LABEL:-latest}
docker login -u $DOCKER_USERNAME -p $DOCKER_TOKEN $DOCKER_REGISTRY

IMG=${DOCKER_REGISTRY}/st4sd-runtime-k8s:${LABEL}-`arch` \
       BUILDER_IMG=${DOCKER_REGISTRY}/mirror/golang:1.19 \
       RUNTIME_IMG=${DOCKER_REGISTRY}/mirror/distroless:nonroot \
       make docker-build

if [ "${DO_PUSH}" == "yes" ]; then
  docker push "$DOCKER_REGISTRY/st4sd-runtime-k8s:${LABEL}-`arch`"
fi
