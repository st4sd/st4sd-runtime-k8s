# Copyright IBM Inc. All Rights Reserved.
# SPDX-License-Identifier: Apache-2.0

# Build the manager binary
ARG go_arch=amd64
ARG builder_image=quay.io/projectquay/golang:1.19
ARG runtime_image=gcr.io/distroless/static:nonroot

FROM ${builder_image} as builder

WORKDIR /workspace
# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum
# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN go mod download

# Copy the go source
COPY main.go main.go
COPY api/ api/
COPY controllers/ controllers/

# Build
RUN CGO_ENABLED=0 GOOS=linux GOARCH=${go_arch} go build -a -o manager main.go

RUN echo "This container is built on top of https://github.com/GoogleContainerTools/distroless/ \n\
(${runtime_image}) \n\
\n\
If you would like the source to the GPL packages in this image then \n\
send a request to this address, specifying the package you want and \n\
the name and hash of this image: \n\
\n\
IBM Research Ireland,\n\
IBM Technology Campus\n\
Damastown Industrial Park\n\
Mulhuddart Co. Dublin D15 HN66\n\
Ireland\n" >/gpl-licenses

# Use distroless as minimal base image to package the manager binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
FROM ${runtime_image}
WORKDIR /
COPY --from=builder /workspace/manager .
COPY --from=builder /gpl-licenses /gpl-licenses

USER 65532:65532

ENTRYPOINT ["/manager"]
