#!/bin/bash
# Diagnostic script to check RocksDB dependencies

echo "======================================"
echo "RocksDB Dependencies Diagnostic"
echo "======================================"
echo ""

# Check if in nix-shell
if [ -n "$IN_NIX_SHELL" ]; then
    echo "✓ Running in nix-shell: $IN_NIX_SHELL"
else
    echo "✗ Not in nix-shell (consider running: nix-shell)"
fi
echo ""

# Check pkg-config path
echo "PKG_CONFIG_PATH: $PKG_CONFIG_PATH"
if [ -z "$PKG_CONFIG_PATH" ]; then
    echo "  (not set - will use: $HOME/.nix-profile/lib/pkgconfig)"
    export PKG_CONFIG_PATH="$HOME/.nix-profile/lib/pkgconfig"
fi
echo ""

# Check for RocksDB
echo "Checking for RocksDB..."
if pkg-config --exists rocksdb 2>/dev/null; then
    echo "✓ RocksDB found via pkg-config"
    echo "  Version: $(pkg-config --modversion rocksdb)"
    echo "  CFLAGS: $(pkg-config --cflags rocksdb)"
    echo "  LIBS: $(pkg-config --libs rocksdb)"
else
    echo "✗ RocksDB not found via pkg-config"
    echo "  Install with: nix-env -iA nixpkgs.rocksdb"
fi
echo ""

# Check for compression libraries
echo "Checking compression libraries..."
for lib in zstd lz4 snappy bz2 z; do
    found=false
    
    # Check via pkg-config
    if pkg-config --exists $lib 2>/dev/null; then
        echo "✓ $lib found via pkg-config"
        found=true
    # Check in nix profile
    elif [ -f "$HOME/.nix-profile/lib/lib${lib}.dylib" ] || [ -f "$HOME/.nix-profile/lib/lib${lib}.so" ] || [ -f "$HOME/.nix-profile/lib/lib${lib}.a" ]; then
        echo "✓ $lib found in $HOME/.nix-profile/lib/"
        found=true
    # Check in system paths
    elif [ -f "/usr/lib/lib${lib}.dylib" ] || [ -f "/usr/lib/lib${lib}.so" ] || [ -f "/usr/local/lib/lib${lib}.dylib" ]; then
        echo "✓ $lib found in system paths"
        found=true
    fi
    
    if [ "$found" = false ]; then
        echo "✗ $lib NOT FOUND"
        echo "  Install with: nix-env -iA nixpkgs.$lib"
    fi
done
echo ""

# Show library directory contents
echo "Libraries in $HOME/.nix-profile/lib/:"
if [ -d "$HOME/.nix-profile/lib" ]; then
    ls -1 $HOME/.nix-profile/lib/ | grep -E "(libzstd|liblz4|libsnappy|libbz2|libz|librocksdb)" | head -20
    echo ""
else
    echo "  Directory not found"
    echo ""
fi

# Test command suggestion
echo "======================================"
echo "Suggested Actions:"
echo "======================================"
echo ""

missing_count=0
for lib in zstd lz4 snappy; do
    if ! pkg-config --exists $lib 2>/dev/null && [ ! -f "$HOME/.nix-profile/lib/lib${lib}.dylib" ] && [ ! -f "$HOME/.nix-profile/lib/lib${lib}.so" ] && [ ! -f "$HOME/.nix-profile/lib/lib${lib}.a" ]; then
        ((missing_count++))
    fi
done

if [ $missing_count -gt 0 ]; then
    echo "Some libraries are missing. Install them with:"
    echo ""
    echo "New Nix (recommended):"
    echo "  nix profile install nixpkgs#zstd nixpkgs#lz4 nixpkgs#bzip2"
    echo ""
    echo "Or old Nix:"
    echo "  nix-env -iA nixpkgs.zstd nixpkgs.lz4 nixpkgs.bzip2"
    echo ""
    echo "Or enter nix-shell (easiest):"
    echo "  nix-shell"
    echo ""
else
    echo "All libraries appear to be installed!"
    echo ""
    echo "Try running the test with:"
    echo ""
    echo "  ./cmd/cronosd/dbmigrate/test-rocksdb.sh"
    echo ""
    echo "Or manually:"
    echo ""
    echo "  export PKG_CONFIG_PATH=\"\$HOME/.nix-profile/lib/pkgconfig\""
    echo "  export CGO_ENABLED=1"
    echo "  export CGO_LDFLAGS=\"-L\$HOME/.nix-profile/lib -lrocksdb -lzstd -llz4 -lsnappy -lbz2 -lz\""
    echo "  go test -v -tags rocksdb ./cmd/cronosd/dbmigrate/..."
    echo ""
fi

echo "======================================"

