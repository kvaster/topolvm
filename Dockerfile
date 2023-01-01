# Build Container
FROM --platform=$BUILDPLATFORM golang:1.19-alpine AS build-env

# Get argment
ARG TOPOLS_VERSION
ARG TARGETARCH

COPY . /workdir
WORKDIR /workdir

RUN apk add --update make curl bash \
    && touch csi/*.go docs/*.md \
    && make build-topols TOPOLS_VERSION=${TOPOLS_VERSION} GOARCH=${TARGETARCH}

# TopoLS container
FROM --platform=$TARGETPLATFORM alpine:edge

RUN apk add --no-cache btrfs-progs

COPY --from=build-env /workdir/build/hypertopols /hypertopols

RUN ln -s hypertopols /topols-scheduler \
    && ln -s hypertopols /topols-node \
    && ln -s hypertopols /topols-controller

COPY --from=build-env /workdir/LICENSE /LICENSE

ENTRYPOINT ["/hypertopols"]
