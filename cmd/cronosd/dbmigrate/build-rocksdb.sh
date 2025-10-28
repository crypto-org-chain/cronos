#!/bin/bash
# Helper script to build cronosd with RocksDB support

set -e

echo "Building cronosd with RocksDB support..."

# Set up pkg-config path
export PKG_CONFIG_PATH="$HOME/.nix-profile/lib/pkgconfig"

# Check if pkg-config can find rocksdb
if ! pkg-config --exists rocksdb; then
    echo "Error: pkg-config cannot find rocksdb"
    echo ""
    echo "Options to install RocksDB:"
    echo ""
    echo "1. Using nix-shell (recommended):"
    echo "   nix-shell"
    echo "   # Then run this script again"
    echo ""
    echo "2. Using new Nix:"
    echo "   nix profile install nixpkgs#rocksdb nixpkgs#zstd nixpkgs#lz4 nixpkgs#bzip2"
    echo ""
    echo "3. Using old Nix:"
    echo "   nix-env -iA nixpkgs.rocksdb nixpkgs.zstd nixpkgs.lz4 nixpkgs.snappy"
    echo ""
    echo "4. Check if already in nix-shell:"
    echo "   echo \$IN_NIX_SHELL"
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
for lib in zstd lz4 snappy bz2 z; do
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

# Check for required dependencies
missing_deps=()
for lib in zstd lz4 snappy; do
    if ! pkg-config --exists $lib 2>/dev/null && [ ! -f "$HOME/.nix-profile/lib/lib${lib}.a" ] && [ ! -f "$HOME/.nix-profile/lib/lib${lib}.dylib" ]; then
        missing_deps+=($lib)
    fi
done

if [ ${#missing_deps[@]} -gt 0 ]; then
    echo "Warning: Missing dependencies: ${missing_deps[*]}"
    echo ""
    echo "Install with new Nix:"
    echo "  nix profile install $(printf 'nixpkgs#%s ' "${missing_deps[@]}")"
    echo ""
    echo "Or old Nix:"
    echo "  nix-env -iA $(printf 'nixpkgs.%s ' "${missing_deps[@]}")"
    echo ""
    echo "Continuing anyway, but build may fail..."
    echo ""
fi

# Get the project root (3 levels up from this script)
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../../.." && pwd)"

cd "$PROJECT_ROOT"

# Build
echo "Building in: $PROJECT_ROOT"
go build -mod=mod -tags rocksdb -o ./cronosd ./cmd/cronosd

echo ""
echo "✅ Build successful!"
echo ""
echo "Binary location: ./cronosd"
echo ""
echo "Test the migration command:"
echo "  ./cronosd migrate-db --help"

