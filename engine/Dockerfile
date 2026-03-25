FROM rust:1.85-slim AS builder

WORKDIR /app
RUN apt-get update && apt-get install -y pkg-config libssl-dev && rm -rf /var/lib/apt/lists/*

# Cache dependencies
COPY Cargo.toml Cargo.lock ./
COPY crates/garance-pooler/Cargo.toml crates/garance-pooler/Cargo.toml
COPY crates/garance-engine/Cargo.toml crates/garance-engine/Cargo.toml
COPY crates/garance-codegen/Cargo.toml crates/garance-codegen/Cargo.toml
RUN mkdir -p crates/garance-pooler/src crates/garance-engine/src crates/garance-codegen/src \
    && echo "pub fn version() -> &'static str { \"0\" }" > crates/garance-pooler/src/lib.rs \
    && echo "pub fn version() -> &'static str { \"0\" }" > crates/garance-engine/src/lib.rs \
    && echo "fn main() {}" > crates/garance-engine/src/main.rs \
    && echo "pub fn version() -> &'static str { \"0\" }" > crates/garance-codegen/src/lib.rs \
    && cargo build --release && rm -rf crates/

# Build real source
COPY crates/ crates/
RUN touch crates/garance-pooler/src/lib.rs \
         crates/garance-codegen/src/lib.rs \
         crates/garance-engine/src/lib.rs \
         crates/garance-engine/src/main.rs \
    && cargo build --release

FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y ca-certificates && rm -rf /var/lib/apt/lists/*
COPY --from=builder /app/target/release/garance-engine /usr/local/bin/garance-engine

ENV LISTEN_ADDR=0.0.0.0:4000
EXPOSE 4000

CMD ["garance-engine"]
