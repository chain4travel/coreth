#!/usr/bin/env bash

# Set the PATHS
GOPATH="$(go env GOPATH)"

# Set binary location
binary_path=${CAMINOETHVM_BINARY_PATH:-"$GOPATH/src/github.com/chain4travel/camino-node/build/plugins/evm"}

# Avalabs docker hub
dockerhub_repo="c4tplatform/caminogo"

# Current tag
caminoethvm_tag=${CURRENT_BRANCH:-$(git describe --tags --exact-match 2> /dev/null || git symbolic-ref -q --short HEAD || git rev-parse --short HEAD)}
echo "Using tag: ${caminoethvm_tag}"

# Image build id
caminoethvm_commit=${CAMINOETHVM_COMMIT:-$( git rev-list -1 HEAD )}
# Use an abbreviated version of the full commit to tag the image.
caminoethvm_short_commit="${caminoethvm_commit::8}"
caminoethvm_tag=${CAMINOETHVM_TAG:-$( git describe --tags --dirty )}

# caminogo version
module=$(grep caminogo $CAMINOETHVM_PATH/go.mod)
# trim leading
module="${module#"${module%%[![:space:]]*}"}"
t=(${module//\ / })
caminogo_tag=${t[-1]}

build_image_id=${BUILD_IMAGE_ID:-"$caminogo_tag-$caminoethvm_short_commit"}
