#!/bin/bash

# Reference:
# https://github.com/golang/go/blob/master/src/go/build/syslist.go
os_archs=(
    aix/ppc64
    darwin/386
    darwin/amd64
    dragonfly/amd64
    freebsd/386
    freebsd/amd64
    freebsd/arm
    freebsd/arm64
    illumos/amd64
    js/wasm
    linux/386
    linux/amd64
    linux/arm
    linux/arm64
    linux/ppc64
    linux/ppc64le
    linux/mips
    linux/mipsle
    linux/mips64
    linux/mips64le
    linux/riscv64
    linux/s390x
    netbsd/386
    netbsd/amd64
    netbsd/arm
    netbsd/arm64
    openbsd/386
    openbsd/amd64
    openbsd/arm
    openbsd/arm64
    plan9/386
    plan9/amd64
    plan9/arm
    solaris/amd64
    windows/386
    windows/amd64
    windows/arm
)

os_archs_32=()
os_archs_64=()

for os_arch in "${os_archs[@]}"
do
    goos=${os_arch%/*}
    goarch=${os_arch#*/}
    GOOS=${goos} GOARCH=${goarch} go build -o ./bin syslog_ng_exporter.go >/dev/null 2>&1 
    if [ $? -eq 0 ]
    then
        os_archs_64+=(${os_arch})
    else
        os_archs_32+=(${os_arch})
    fi
done

echo "32-bit:"
for os_arch in "${os_archs_32[@]}"
do
    printf "\t%s\n" "${os_arch}"
done
echo

echo "64-bit:"
for os_arch in "${os_archs_64[@]}"
do
    printf "\t%s\n" "${os_arch}"
done
echo