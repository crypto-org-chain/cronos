{
  lib,
  stdenv,
  fetchFromGitHub,
  fetchpatch,
  cmake,
  ninja,
  pkg-config,
  bzip2,
  lz4,
  snappy,
  zlib,
  zstd,
  windows,
  # only enable jemalloc for non-windows platforms
  # see: https://github.com/NixOS/nixpkgs/issues/216479
  enableJemalloc ? !stdenv.hostPlatform.isWindows && !stdenv.hostPlatform.isStatic,
  jemalloc,
  enableLite ? false,
  enableShared ? !stdenv.hostPlatform.isStatic,
  sse42Support ? stdenv.hostPlatform.sse4_2Support,
}:

stdenv.mkDerivation rec {
  pname = "rocksdb";
  version = "10.5.1";

  withLz4 = true;

  src = fetchFromGitHub {
    owner = "facebook";
    repo = pname;
    rev = "v${version}";
    sha256 = "sha256-TDYXzYbOLhcIRi+qi0FW1OLVtfKOF+gUbj62Tgpp3/E=";
  };

  nativeBuildInputs = [
    cmake
    ninja
    pkg-config
  ];

  propagatedBuildInputs = [
    bzip2
    snappy
    zlib
    zstd
  ]
  ++ lib.optional withLz4 lz4;

  buildInputs =
    lib.optional withLz4 lz4
    ++ lib.optional enableJemalloc jemalloc
    ++ lib.optional stdenv.hostPlatform.isMinGW windows.pthreads;

  outputs = [
    "out"
    "tools"
  ];

  NIX_CFLAGS_COMPILE =
    lib.optionals stdenv.cc.isGNU [
      "-Wno-error=deprecated-copy"
      "-Wno-error=pessimizing-move"
      # Needed with GCC 12
      "-Wno-error=format-truncation"
      "-Wno-error=maybe-uninitialized"
    ]
    ++ lib.optionals stdenv.cc.isClang [
      "-Wno-error=unused-private-field"
      "-Wno-nontrivial-memcall"
      "-faligned-allocation"
    ];

  cmakeFlags = [
    "-DPORTABLE=1"
    "-DWITH_JEMALLOC=${if enableJemalloc then "1" else "0"}"
    "-DWITH_JNI=0"
    "-DWITH_BENCHMARK_TOOLS=0"
    "-DWITH_TESTS=1"
    "-DWITH_TOOLS=0"
    "-DWITH_CORE_TOOLS=1"
    "-DWITH_BZ2=1"
    "-DWITH_LZ4=${if withLz4 then "1" else "0"}"
    "-DWITH_SNAPPY=1"
    "-DWITH_ZLIB=1"
    "-DWITH_ZSTD=1"
    "-DWITH_GFLAGS=0"
    "-DUSE_RTTI=1"
    "-DROCKSDB_INSTALL_ON_WINDOWS=YES" # harmless elsewhere
    (lib.optional sse42Support "-DFORCE_SSE42=1")
    (lib.optional enableLite "-DROCKSDB_LITE=1")
    "-DFAIL_ON_WARNINGS=${if stdenv.hostPlatform.isMinGW then "NO" else "YES"}"
  ]
  ++ lib.optionals stdenv.hostPlatform.isMinGW [
    # Let pkg-config drive LZ4 discovery on MinGW; the CMake package file
    # can point at a stub import library, while pkg-config resolves to the
    # full import library output.
    "-DCMAKE_REQUIRE_FIND_PACKAGE_PkgConfig=ON"

  ]
  ++ lib.optional (!enableShared) "-DROCKSDB_BUILD_SHARED=0";

  # otherwise "cc1: error: -Wformat-security ignored without -Wformat [-Werror=format-security]"
  hardeningDisable = lib.optional stdenv.hostPlatform.isWindows "format";

  postPatch = ''
    substituteInPlace port/mmap.cc \
      --replace 'std::memcpy(this, &other, sizeof(*this));' \
      'std::memcpy(static_cast<void*>(this), static_cast<const void*>(&other), sizeof(*this));'
  '';

  preConfigure = lib.optionalString (stdenv.hostPlatform.isMinGW && withLz4) ''
            # The MinGW lz4 package ships a stub import library that points at the
            # executable instead of the DLL. Generate a correct import library from
            # the actual DLL so RocksDB can link with LZ4.
            lz4_out=${if lz4 ? out then lz4.out else lz4}
            lz4_dev=${if lz4 ? dev then lz4.dev else lz4}
            lz4_dll=$lz4_out/bin/liblz4.dll
            lz4_import_dir=$PWD/lz4-import
            mkdir -p "$lz4_import_dir/lib" "$lz4_import_dir/lib/pkgconfig" "$lz4_import_dir/lib/cmake/lz4"

            {
              echo "LIBRARY liblz4.dll"
              echo "EXPORTS"
              ${stdenv.cc.bintools.targetPrefix}objdump -p "$lz4_dll" \
                | awk '/\\+base/ && $NF ~ /^LZ4/ {print $NF}'
            } > "$lz4_import_dir/lz4.def"

            ${stdenv.cc.bintools.targetPrefix}dlltool \
              --dllname liblz4.dll \
              --def "$lz4_import_dir/lz4.def" \
              --output-lib "$lz4_import_dir/lib/liblz4.dll.a"

            cat > "$lz4_import_dir/lib/pkgconfig/liblz4.pc" <<EOF
    prefix=$lz4_out
    exec_prefix=$lz4_out
    libdir=$lz4_import_dir/lib
    includedir=$lz4_dev/include

    Name: lz4
    Description: LZ4 import library regenerated for MinGW
    Version: 1.10.0
    Libs: -L$libdir -llz4
    Cflags: -I$includedir
    EOF

            cat > "$lz4_import_dir/lib/cmake/lz4/lz4Config.cmake" <<EOF
    set(LZ4_FOUND TRUE)
    set(LZ4_INCLUDE_DIR "$lz4_dev/include")
    set(LZ4_LIBRARIES "$lz4_import_dir/lib/liblz4.dll.a")
    add_library(lz4::lz4 SHARED IMPORTED)
    set_target_properties(lz4::lz4 PROPERTIES
      IMPORTED_IMPLIB "$lz4_import_dir/lib/liblz4.dll.a"
      IMPORTED_LOCATION "$lz4_out/bin/liblz4.dll"
      INTERFACE_INCLUDE_DIRECTORIES "$lz4_dev/include"
    )
    EOF

            export PKG_CONFIG_PATH="$lz4_import_dir/lib/pkgconfig''${PKG_CONFIG_PATH:+:}$PKG_CONFIG_PATH"
            export CMAKE_LIBRARY_PATH="$lz4_import_dir/lib''${CMAKE_LIBRARY_PATH:+:}$CMAKE_LIBRARY_PATH"
            export CMAKE_INCLUDE_PATH="$lz4_dev/include''${CMAKE_INCLUDE_PATH:+:}$CMAKE_INCLUDE_PATH"
            export LZ4_DIR="$lz4_import_dir/lib/cmake/lz4"
            export lz4_DIR="$lz4_import_dir/lib/cmake/lz4"
            export CMAKE_PREFIX_PATH="$lz4_import_dir''${CMAKE_PREFIX_PATH:+:}$CMAKE_PREFIX_PATH"

            cmakeFlagsArray+=(
              "-DLZ4_LIBRARY=$lz4_import_dir/lib/liblz4.dll.a"
              "-DLZ4_LIBRARIES=$lz4_import_dir/lib/liblz4.dll.a"
              "-DLZ4_INCLUDE_DIR=$lz4_dev/include"
            )
  '';

  preInstall = ''
    mkdir -p $tools/bin
    cp tools/{ldb,sst_dump}${stdenv.hostPlatform.extensions.executable} $tools/bin/
  ''
  + lib.optionalString stdenv.isDarwin ''
    ls -1 $tools/bin/* | xargs -I{} ${stdenv.cc.bintools.targetPrefix}install_name_tool -change "@rpath/librocksdb.${lib.versions.major version}.dylib" $out/lib/librocksdb.dylib {}
  ''
  + lib.optionalString (stdenv.isLinux && enableShared) ''
    ls -1 $tools/bin/* | xargs -I{} patchelf --set-rpath $out/lib:${stdenv.cc.cc.lib}/lib {}
  '';

  # Old version doesn't ship the .pc file, new version puts wrong paths in there.
  postFixup = ''
    if [ -f "$out"/lib/pkgconfig/rocksdb.pc ]; then
      substituteInPlace "$out"/lib/pkgconfig/rocksdb.pc \
        --replace '="''${prefix}//' '="/'
    fi
  ''
  + lib.optionalString stdenv.isDarwin ''
    ${stdenv.cc.targetPrefix}install_name_tool -change "@rpath/libsnappy.1.dylib" "${snappy}/lib/libsnappy.1.dylib" $out/lib/librocksdb.dylib
    ${stdenv.cc.targetPrefix}install_name_tool -change "@rpath/librocksdb.${lib.versions.major version}.dylib" "$out/lib/librocksdb.${lib.versions.major version}.dylib" $out/lib/librocksdb.dylib
  '';

  meta = with lib; {
    homepage = "https://rocksdb.org";
    description = "A library that provides an embeddable, persistent key-value store for fast storage";
    changelog = "https://github.com/facebook/rocksdb/raw/v${version}/HISTORY.md";
    license = licenses.asl20;
    platforms = platforms.all;
    maintainers = with maintainers; [
      adev
      magenbluten
    ];
  };
}
