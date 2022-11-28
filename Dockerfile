
# Simple usage with a mounted data directory:
# > docker build -t cryptocom/cronos .
# > docker run -it -p 26657:26657 -p 26656:26656 -v ~/.cronos:/cronos/.cronos cryptocom/cronos cronosd start
FROM golang:alpine AS build-env

ARG NETWORK=testnet

# Set up dependencies
ENV PACKAGES curl libc-dev make git bash gcc linux-headers eudev-dev python3

# Set working directory for the build
WORKDIR /go/src/github.com/crypto-org-chain/cronos

# Add source files
COPY . .

# Install minimum necessary dependencies, build Cosmos SDK, remove packages
RUN apk add --no-cache $PACKAGES && \
  git submodule update --init --recursive && \
  NETWORK=${NETWORK} make install

# Final image
FROM alpine:edge

ENV CRONOS /cronos

# Install ca-certificates
RUN apk add --update ca-certificates

RUN addgroup cronos && \
  adduser -S -G cronos cronos -h "$CRONOS"

USER cronos

WORKDIR $CRONOS

# Copy over binaries from the build-env
COPY --from=build-env /go/bin/cronosd /usr/bin/cronosd

# Run cronosd by default
CMD ["cronosd"]