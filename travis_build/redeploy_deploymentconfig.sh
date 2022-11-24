#!/usr/bin/env bash
#
# Copyright IBM Inc. All Rights Reserved.
# SPDX-License-Identifier: Apache-2.0
# Authors:
#  Vassilis Vassiliadis
#  Alessandro Pomponio

set +x

export PROJECT_NAME=$1
export OC_LOGIN_URL=$2
export OC_LOGIN_TOKEN=$3

wget https://mirror.openshift.com/pub/openshift-v4/clients/ocp/latest/openshift-client-linux.tar.gz
tar -xvf openshift-client-linux.tar.gz
rm openshift-client-linux.tar.gz

chmod +x oc kubectl
export PATH=$PATH:${PWD}
oc login "${OC_LOGIN_URL}" --token "${OC_LOGIN_TOKEN}" --insecure-skip-tls-verify=true

# VV: Delete the `st4sd-runtime-k8s' pod
# ImageStreamTags assume that the manifest they point to never gets garbage collected.
# We cannot guarantee that this is the case so we do not use ImageStreamTags in development environments.
# We trigger re-deployment simply by deleting the pod of the `st4sd-runtime-k8s` DeploymentConfig,
# the associated replication controller will create a new one for us.
oc project ${PROJECT_NAME}
oc delete pod -ldeploymentconfig=st4sd-runtime-k8s
