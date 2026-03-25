# Build from repo root: docker build -f engine/Dockerfile .
FROM rust:1.85-slim AS builder

WORKDIR /build
RUN apt-get update && apt-get install -y pkg-config libssl-dev protobuf-compiler && rm -rf /var/lib/apt/lists/*

# Copy proto files (needed by tonic-build in build.rs)
COPY proto/engine/v1/ ./proto/engine/v1/

# Cache dependencies
COPY engine/Cargo.toml engine/Cargo.lock ./engine/
COPY engine/crates/garance-pooler/Cargo.toml ./engine/crates/garance-pooler/Cargo.toml
COPY engine/crates/garance-engine/Cargo.toml ./engine/crates/garance-engine/Cargo.toml
COPY engine/crates/garance-codegen/Cargo.toml ./engine/crates/garance-codegen/Cargo.toml

WORKDIR /build/engine
RUN mkdir -p crates/garance-pooler/src crates/garance-engine/src crates/garance-codegen/src \
    && echo "pub fn version() -> &'static str { \"0\" }" > crates/garance-pooler/src/lib.rs \
    && echo "pub fn version() -> &'static str { \"0\" }" > crates/garance-engine/src/lib.rs \
    && echo "fn main() {}" > crates/garance-engine/src/main.rs \
    && echo "pub fn version() -> &'static str { \"0\" }" > crates/garance-codegen/src/lib.rs \
    && cargo build --release 2>/dev/null || true

# Build real source
WORKDIR /build
COPY engine/crates/ ./engine/crates/
WORKDIR /build/engine
RUN touch crates/garance-pooler/src/lib.rs \
         crates/garance-codegen/src/lib.rs \
         crates/garance-engine/src/lib.rs \
         crates/garance-engine/src/main.rs \
    && cargo build --release

FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y ca-certificates && rm -rf /var/lib/apt/lists/*
COPY --from=builder /build/engine/target/release/garance-engine /usr/local/bin/garance-engine

ENV LISTEN_ADDR=0.0.0.0:4000
EXPOSE 4000
CMD ["garance-engine"]
