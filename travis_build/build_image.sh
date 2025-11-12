#!/usr/bin/env bash
#
# Copyright IBM Inc. All Rights Reserved.
# SPDX-License-Identifier: Apache-2.0
# Authors:
#  Vassilis Vassiliadis


set -euxo pipefail -o xtrace

export DO_PUSH=${DO_PUSH:-yes}
export LABEL=${LABEL:-latest}
docker login -u ${DOCKER_USERNAME} -p ${DOCKER_TOKEN} ${DOCKER_REGISTRY}

IMG=${IMAGE_BASE_URL}:${LABEL}-`arch` \
       make docker-build

if [ "${DO_PUSH}" == "yes" ]; then
  docker push "${IMAGE_BASE_URL}:${LABEL}-`arch`"
fi
