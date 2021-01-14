# Build Container
FROM golang:1.15-alpine AS build-env

# Get argment
ARG TOPOLS_VERSION

COPY . /workdir
WORKDIR /workdir

RUN apk add --update make curl bash && make build TOPOLS_VERSION=${TOPOLS_VERSION}

# TopoLS container
FROM alpine:edge

RUN apk add --no-cache btrfs-progs

COPY --from=build-env /workdir/build/hypertopols /hypertopols

RUN ln -s hypertopols /topols-scheduler \
    && ln -s hypertopols /topols-node \
    && ln -s hypertopols /topols-controller

# CSI sidecar
COPY --from=build-env /workdir/build/csi-provisioner /csi-provisioner
COPY --from=build-env /workdir/build/csi-node-driver-registrar /csi-node-driver-registrar
COPY --from=build-env /workdir/build/csi-attacher /csi-attacher
COPY --from=build-env /workdir/build/csi-resizer /csi-resizer
COPY --from=build-env /workdir/build/livenessprobe /livenessprobe
COPY --from=build-env /workdir/LICENSE /LICENSE

ENTRYPOINT ["/hypertopols"]
