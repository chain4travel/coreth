# ============= Compilation Stage ================
FROM golang:1.17.9-buster AS builder

RUN apt-get update && apt-get install -y --no-install-recommends bash=5.0-4 git=1:2.20.1-2+deb10u3 make=4.2.1-1.2 gcc=4:8.3.0-1 musl-dev=1.1.21-2 ca-certificates=20200601~deb10u2 linux-headers-amd64

ARG CAMINO_VERSION

RUN mkdir -p $GOPATH/src/github.com/ava-labs
WORKDIR $GOPATH/src/github.com/ava-labs

RUN git clone -b $CAMINO_VERSION --single-branch https://github.com/chain4travel/caminogo.git

# Copy coreth repo into desired location
COPY . coreth

# Set the workdir to AvalancheGo and update coreth dependency to local version
WORKDIR $GOPATH/src/github.com/chain4travel/caminogo
# Run go mod download here to improve caching of AvalancheGo specific depednencies
RUN go mod download
# Replace the coreth dependency
RUN go mod edit -replace github.com/chain4travel/caminoethvm=../coreth
RUN go mod download

# Build the AvalancheGo binary with local version of coreth.
RUN ./scripts/build_camino.sh
# Create the plugins directory in the standard location so the build directory will be recognized
# as valid.
RUN mkdir build/plugins

# ============= Cleanup Stage ================
FROM debian:11-slim AS execution

# Maintain compatibility with previous images
RUN mkdir -p /caminogo/build
WORKDIR /caminogo/build

# Copy the executables into the container
COPY --from=builder /go/src/github.com/chain4travel/caminogo/build .

CMD [ "./caminogo" ]
