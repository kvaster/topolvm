# Build Container
FROM golang:1.15-alpine AS build-env

# Get argment
ARG TOPOLVM_VERSION

COPY . /workdir
WORKDIR /workdir

RUN apk add --update make curl bash && make build TOPOLVM_VERSION=${TOPOLVM_VERSION}

# TopoLVM container
FROM alpine:edge

ENV DEBIAN_FRONTEND=noninteractive
RUN apk add --no-cache btrfs-progs

COPY --from=build-env /workdir/build/hypertopolvm /hypertopolvm

RUN ln -s hypertopolvm /lvmd \
    && ln -s hypertopolvm /topolvm-scheduler \
    && ln -s hypertopolvm /topolvm-node \
    && ln -s hypertopolvm /topolvm-controller

# CSI sidecar
COPY --from=build-env /workdir/build/csi-provisioner /csi-provisioner
COPY --from=build-env /workdir/build/csi-node-driver-registrar /csi-node-driver-registrar
COPY --from=build-env /workdir/build/csi-attacher /csi-attacher
COPY --from=build-env /workdir/build/csi-resizer /csi-resizer
COPY --from=build-env /workdir/build/livenessprobe /livenessprobe
COPY --from=build-env /workdir/LICENSE /LICENSE

ENTRYPOINT ["/hypertopolvm"]
