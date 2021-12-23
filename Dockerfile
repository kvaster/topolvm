# Build Container
FROM golang:1.17-alpine AS build-env

# Get argment
ARG TOPOLS_VERSION

COPY . /workdir
WORKDIR /workdir

RUN touch csi/*.go lvmd/proto/*.go docs/*.md \
    && apk add --update make curl bash \
    && make build TOPOLS_VERSION=${TOPOLS_VERSION}

# TopoLS container
FROM alpine:edge

RUN apk add --no-cache btrfs-progs

COPY --from=build-env /workdir/build/hypertopols /hypertopols

RUN ln -s hypertopols /topols-scheduler \
    && ln -s hypertopols /topols-node \
    && ln -s hypertopols /topols-controller

COPY --from=build-env /workdir/LICENSE /LICENSE

ENTRYPOINT ["/hypertopols"]
