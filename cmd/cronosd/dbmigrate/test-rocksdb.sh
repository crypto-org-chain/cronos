#!/bin/bash
# Helper script to run RocksDB tests with proper environment setup

set -e

echo "Setting up RocksDB environment for Nix..."

# Set up pkg-config path
export PKG_CONFIG_PATH="$HOME/.nix-profile/lib/pkgconfig"

# Check if pkg-config can find rocksdb
if ! pkg-config --exists rocksdb; then
    echo "Error: pkg-config cannot find rocksdb"
    echo "Please ensure RocksDB is installed:"
    echo ""
    echo "Option 1 - Use nix-shell (recommended):"
    echo "  nix-shell"
    echo ""
    echo "Option 2 - Install with new Nix:"
    echo "  nix profile install nixpkgs#rocksdb nixpkgs#zstd nixpkgs#lz4 nixpkgs#bzip2"
    echo ""
    echo "Option 3 - Install with old Nix:"
    echo "  nix-env -iA nixpkgs.rocksdb nixpkgs.zstd"
    echo ""
    exit 1
fi

# Set up CGO flags
export CGO_ENABLED=1
export CGO_CFLAGS="$(pkg-config --cflags rocksdb)"

# Build LDFLAGS with all dependencies
LDFLAGS="$(pkg-config --libs rocksdb)"

# Add explicit library paths and dependencies for nix
if [ -d "$HOME/.nix-profile/lib" ]; then
    LDFLAGS="$LDFLAGS -L$HOME/.nix-profile/lib"
fi

# Add common RocksDB dependencies explicitly
for lib in snappy z; do
    if pkg-config --exists $lib 2>/dev/null; then
        LDFLAGS="$LDFLAGS $(pkg-config --libs $lib)"
    elif [ -f "$HOME/.nix-profile/lib/lib${lib}.a" ] || [ -f "$HOME/.nix-profile/lib/lib${lib}.dylib" ] || [ -f "$HOME/.nix-profile/lib/lib${lib}.so" ]; then
        LDFLAGS="$LDFLAGS -l${lib}"
    fi
done

export CGO_LDFLAGS="$LDFLAGS"

echo "Environment configured:"
echo "  PKG_CONFIG_PATH=$PKG_CONFIG_PATH"
echo "  CGO_CFLAGS=$CGO_CFLAGS"
echo "  CGO_LDFLAGS=$CGO_LDFLAGS"
echo ""

# Check for zstd specifically since it's a common issue
#if ! pkg-config --exists zstd && [ ! -f "$HOME/.nix-profile/lib/libzstd.a" ] && [ ! -f "$HOME/.nix-profile/lib/libzstd.dylib" ]; then
#    echo "Warning: zstd library not found"
#    echo "Install with: nix profile install nixpkgs#zstd"
#    echo "Or old Nix: nix-env -iA nixpkgs.zstd"
#    echo ""
#fi

# Run tests
echo "Running RocksDB tests..."
go test -mod=mod -v -tags rocksdb ./cmd/cronosd/dbmigrate/... "$@"

