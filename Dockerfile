# Build Container
FROM --platform=$BUILDPLATFORM golang:1.20-alpine AS build-topols

# Get argument
ARG TOPOLS_VERSION
ARG TARGETARCH

COPY . /workdir
WORKDIR /workdir

RUN apk add --update make curl bash \
    && make build-topols TOPOLS_VERSION=${TOPOLS_VERSION} GOARCH=${TARGETARCH}

# TopoLS container
FROM --platform=$TARGETPLATFORM alpine:edge AS topols

RUN apk add --no-cache btrfs-progs

COPY --from=build-topols /workdir/build/hypertopols /hypertopols

RUN ln -s hypertopols /topols-scheduler \
    && ln -s hypertopols /topols-node \
    && ln -s hypertopols /topols-controller

COPY --from=build-topols /workdir/LICENSE /LICENSE

ENTRYPOINT ["/hypertopols"]

# Build sidecars
FROM --platform=$BUILDPLATFORM build-topols as build-sidecars

# Get argument
ARG TARGETARCH

RUN apk add --update patch \
    && make csi-sidecars GOARCH=${TARGETARCH}

# TopoLS container with sidecar
FROM --platform=$TARGETPLATFORM topols as topols-with-sidecar

COPY --from=build-sidecars /workdir/build/csi-provisioner /csi-provisioner
COPY --from=build-sidecars /workdir/build/csi-node-driver-registrar /csi-node-driver-registrar
COPY --from=build-sidecars /workdir/build/csi-resizer /csi-resizer
COPY --from=build-sidecars /workdir/build/csi-snapshotter /csi-snapshotter
COPY --from=build-sidecars /workdir/build/livenessprobe /livenessprobe

ENTRYPOINT ["/hypertopols"]
