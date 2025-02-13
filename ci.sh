#!/usr/bin/env bash
set -ex
export GO111MODULE=on
export LD_LIBRARY_PATH=`pwd`/lib/cuckoo/target/release:$LD_LIBRARY_PATH
echo $LD_LIBRARY_PATH
go mod tidy

if [ ! -x "$(type -p golangci-lint)" ]; then
  exit 1
fi

golangci-lint --version
go build
echo -e "\n Success!"


