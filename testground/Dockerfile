# Multi-architecture Dockerfile for cronos-testground
# Supports: linux/amd64, linux/arm64 (Apple M-series CPUs)
#
# Build for current platform:
#   docker build -t cronos-testground .
#
# Build for ARM64 (M1):
#   docker buildx build --platform linux/arm64 -t cronos-testground:arm64 .
#
# Build multi-arch and push:
#   docker buildx build --platform linux/amd64,linux/arm64 -t ghcr.io/crypto-org-chain/cronos-testground:latest --push .

# =============================================================================
# Stage 1: Build RocksDB from source
# =============================================================================
FROM debian:bookworm-slim AS rocksdb-builder

# Install build dependencies for RocksDB
RUN apt-get update && apt-get install -y \
    build-essential \
    cmake \
    ninja-build \
    git \
    ca-certificates \
    libbz2-dev \
    liblz4-dev \
    libsnappy-dev \
    zlib1g-dev \
    libzstd-dev \
    && rm -rf /var/lib/apt/lists/*

# Optional: corporate CA certificate for environments behind TLS-inspecting proxies
# (e.g. Palo Alto GlobalProtect, Zscaler). Only needed if git clone or network
# requests fail with SSL errors during the build. Developers not behind a corporate
# proxy don't need this file at all.
#
# How to obtain your .crt file:
#   1. Ask your IT/security team for the corporate root CA certificate bundle.
#   2. Export from your system keychain — on macOS, open Keychain Access, find the
#      corporate root CA certs, and export them as .pem/.crt.
#   3. Export from your browser — visit any HTTPS site, inspect the certificate
#      chain, and export the root CA.
#
# Place the file at testground/corp-ca.crt before building.
# The wildcard glob makes this a no-op when the file is absent.
COPY testground/corp-ca.crt* /tmp/
RUN if [ -f /tmp/corp-ca.crt ]; then \
        cp /tmp/corp-ca.crt /usr/local/share/ca-certificates/corp-ca.crt && \
        update-ca-certificates; \
    fi

# Build RocksDB 10.6.2 (same version as nix/rocksdb.nix)
WORKDIR /rocksdb
RUN git clone --depth 1 --branch v10.6.2 https://github.com/facebook/rocksdb.git . \
    && sed -i 's/std::memcpy(this, &other, sizeof(\*this));/std::memcpy(static_cast<void*>(this), static_cast<const void*>(\&other), sizeof(*this));/' port/mmap.cc \
    && mkdir build && cd build \
    && cmake .. -G Ninja \
        -DCMAKE_BUILD_TYPE=Release \
        -DCMAKE_INSTALL_PREFIX=/usr \
        -DCMAKE_INSTALL_LIBDIR=lib \
        -DPORTABLE=1 \
        -DWITH_JEMALLOC=0 \
        -DWITH_JNI=0 \
        -DWITH_BENCHMARK_TOOLS=0 \
        -DWITH_TESTS=0 \
        -DWITH_TOOLS=0 \
        -DWITH_CORE_TOOLS=0 \
        -DWITH_BZ2=1 \
        -DWITH_LZ4=1 \
        -DWITH_SNAPPY=1 \
        -DWITH_ZLIB=1 \
        -DWITH_ZSTD=1 \
        -DWITH_GFLAGS=0 \
        -DUSE_RTTI=1 \
        -DFAIL_ON_WARNINGS=NO \
        -DROCKSDB_BUILD_SHARED=ON \
    && ninja \
    && ninja install

# =============================================================================
# Stage 2: Build cronosd
# =============================================================================
FROM golang:1.25-bookworm AS cronosd-builder

# Install build dependencies
RUN apt-get update && apt-get install -y \
    git \
    make \
    gcc \
    g++ \
    libc-dev \
    patch \
    ca-certificates \
    libbz2-dev \
    liblz4-dev \
    libsnappy-dev \
    zlib1g-dev \
    libzstd-dev \
    && rm -rf /var/lib/apt/lists/*

# Copy RocksDB from builder
COPY --from=rocksdb-builder /usr/lib/librocksdb* /usr/lib/
COPY --from=rocksdb-builder /usr/include/rocksdb /usr/include/rocksdb
# Copy corporate CA certificate if it was installed in rocksdb-builder stage
COPY --from=rocksdb-builder /usr/local/share/ca-certificates/corp-ca.crt* /usr/local/share/ca-certificates/
# Verify RocksDB and update CA certs
RUN ldconfig && ls -la /usr/lib/librocksdb* && echo "RocksDB libraries found" \
    && if [ -f /usr/local/share/ca-certificates/corp-ca.crt ]; then update-ca-certificates; fi

WORKDIR /src

# Copy everything
COPY . .

# Apply testground patch for UnsafeUnorderedTx
RUN if [ -f nix/testground-cronosd.patch ]; then \
        git apply nix/testground-cronosd.patch 2>/dev/null || patch -p1 < nix/testground-cronosd.patch; \
    fi

# Build cronosd
# Use -mod=mod to allow dependency resolution (vendor may be out of sync)
ARG VERSION=docker
ARG COMMIT=docker
ENV CGO_CFLAGS="-I/usr/include"
ENV CGO_LDFLAGS="-L/usr/lib -lrocksdb -lstdc++ -lm -lz -lbz2 -lsnappy -llz4 -lzstd"
RUN CGO_ENABLED=1 go build \
    -mod=mod \
    -tags "netgo objstore pebbledb rocksdb grocksdb_clean_link mainnet nativebyteorder" \
    -ldflags "-w -s -X github.com/cosmos/cosmos-sdk/version.Name=cronos \
              -X github.com/cosmos/cosmos-sdk/version.AppName=cronosd \
              -X github.com/cosmos/cosmos-sdk/version.Version=${VERSION} \
              -X github.com/cosmos/cosmos-sdk/version.Commit=${COMMIT}" \
    -trimpath \
    -o /src/build/cronosd \
    ./cmd/cronosd

# =============================================================================
# Stage 3: Build Python benchmark package
# =============================================================================
FROM python:3.11-slim-bookworm AS python-builder

WORKDIR /app

# Install build dependencies (needed for cprotobuf and other native extensions)
RUN apt-get update && apt-get install -y \
    gcc \
    g++ \
    libc-dev \
    && rm -rf /var/lib/apt/lists/*

# Install poetry
RUN pip install --no-cache-dir poetry

# Copy benchmark package files
COPY testground/benchmark/pyproject.toml testground/benchmark/poetry.lock* ./
COPY testground/benchmark/benchmark ./benchmark/

# Install dependencies and build wheel
RUN poetry config virtualenvs.create false \
    && poetry install --no-interaction --no-ansi --only main \
    && poetry build -f wheel

# =============================================================================
# Stage 4: Final image
# =============================================================================
FROM python:3.11-slim-bookworm

# Install runtime dependencies and build tools for native Python extensions
RUN apt-get update && apt-get install -y \
    ca-certificates \
    gcc \
    libc-dev \
    libbz2-1.0 \
    liblz4-1 \
    libsnappy1v5 \
    zlib1g \
    libzstd1 \
    && rm -rf /var/lib/apt/lists/*

# Copy RocksDB runtime libraries
COPY --from=rocksdb-builder /usr/lib/librocksdb* /usr/lib/
RUN ldconfig

# Create tmp directory
RUN mkdir -p /tmp

# Copy cronosd binary
COPY --from=cronosd-builder /src/build/cronosd /bin/cronosd

# Copy Python package wheel and install with dependencies
COPY --from=python-builder /app/dist/*.whl /tmp/
RUN pip install --no-cache-dir /tmp/*.whl && rm /tmp/*.whl

# Clean up build tools to reduce image size (keep librocksdb for runtime)
RUN apt-get purge -y gcc libc-dev && apt-get autoremove -y && apt-get clean

# Expose ports (same as nix/testground-image.nix)
EXPOSE 9090 26657 26656 1317 26658 26660 26659 30000

# Set environment
ENV PYTHONUNBUFFERED=1

# =============================================================================
# Optional: Embed test data during build
# =============================================================================
# Set EMBED_DATA=true to generate and embed test data at build time
# Configuration is read from testground/benchmark-options.json
ARG EMBED_DATA=false

# Copy the benchmark options file
# Edit testground/benchmark-options.json before building to customize:
#   - validators/fullnodes: number of nodes
#   - num_accounts: test accounts per node
#   - num_txs: transactions per account
#   - tx_type: "simple-transfer" or "erc20-transfer"
#   - batch_size: 1 for single, 100 for batch tests
#   - config_patch: CometBFT config.toml overrides
#   - app_patch: Cronos app.toml overrides
#   - genesis_patch: genesis.json overrides
COPY testground/benchmark-options.json /tmp/benchmark-options.json

# Generate and embed test data if EMBED_DATA=true
# Uses the generic-gen command which reads all options from JSON
RUN if [ "$EMBED_DATA" = "true" ]; then \
        echo "Generating test data from /tmp/benchmark-options.json:" && \
        cat /tmp/benchmark-options.json && \
        echo "" && \
        /usr/local/bin/stateless-testcase generic-gen "$(cat /tmp/benchmark-options.json)" && \
        echo "Test data embedded at /data"; \
    else \
        echo "EMBED_DATA not set, skipping test data generation"; \
    fi

# Default command
CMD ["/usr/local/bin/stateless-testcase"]

