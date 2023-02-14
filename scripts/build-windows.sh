#!/bin/bash
uname -a
export GOROOT=/mingw64/lib/go
export GOPATH=$HOME/go
export PATH=$GOPATH/bin:$GOROOT/bin:$PATH             
export CGO_CFLAGS="-I/mingw64/include/rocksdb" 
export CGO_LDFLAGS="-L/mingw64/lib -lrocksdb -lstdc++ -lm -lz -lbz2 -lsnappy -llz4 -lzstd" 
echo $PATH
go version
wget https://mirror.msys2.org/mingw/mingw64/mingw-w64-x86_64-rocksdb-7.9.2-1-any.pkg.tar.zst
pacman -U mingw-w64-x86_64-rocksdb-7.9.2-1-any.pkg.tar.zst --noconfirm
COSMOS_BUILD_OPTIONS=rocksdb make build
ls -la ./build/
mv ./build/cronosd ./build/cronosd.exe
cp /mingw64/bin/libbz2-1.dll ./build 
cp /mingw64/bin/libgcc_s_seh-1.dll ./build
cp /mingw64/bin/liblz4.dll ./build
cp /mingw64/bin/librocksdb.dll ./build
cp /mingw64/bin/libsnappy.dll ./build
cp /mingw64/bin/libstdc++-6.dll ./build
cp /mingw64/bin/libwinpthread-1.dll ./build
cp /mingw64/bin/libzstd.dll ./build
cp /mingw64/bin/zlib1.dll ./build
