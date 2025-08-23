# Multi-stage build for golang-profiling
# Stage 1: Build environment
FROM rust:1.75-alpine as builder

# Install system dependencies
RUN apk add --no-cache \
    build-base \
    pkgconfig \
    openssl-dev \
    clang \
    llvm \
    libbpf-dev \
    linux-headers \
    musl-dev

# Install bpf-linker for eBPF compilation
RUN cargo install bpf-linker

# Set working directory
WORKDIR /usr/src/golang-profiling

# Copy workspace files
COPY Cargo.toml Cargo.lock ./
COPY rustfmt.toml ./

# Copy all project directories
COPY golang-profiling-common/ ./golang-profiling-common/
COPY golang-profiling-ebpf/ ./golang-profiling-ebpf/
COPY golang-profiling/ ./golang-profiling/
COPY flamegraph.pl ./

# Build the project
WORKDIR /usr/src/golang-profiling
RUN rustup target add x86_64-unknown-linux-musl
RUN cargo build --release --target x86_64-unknown-linux-musl -p golang-profiling

# Stage 2: Runtime environment
FROM alpine:3.18

# Install runtime dependencies
RUN apk add --no-cache \
    ca-certificates \
    perl \
    procps

# Create non-root user
RUN adduser -D -s /bin/false golang-profiling

# Copy the binary from builder stage
COPY --from=builder /usr/src/golang-profiling/target/x86_64-unknown-linux-musl/release/golang-profiling /usr/local/bin/

# Copy flamegraph.pl script
COPY --from=builder /usr/src/golang-profiling/flamegraph.pl /usr/local/bin/

# Make scripts executable
RUN chmod +x /usr/local/bin/golang-profiling /usr/local/bin/flamegraph.pl

# Set capabilities for eBPF operations (when running privileged)
# Note: This requires the container to run with --privileged or specific capabilities

# Create output directory
RUN mkdir -p /tmp/profiling && chown golang-profiling:golang-profiling /tmp/profiling

# Set default working directory
WORKDIR /tmp/profiling

# Default user (can be overridden for privileged operations)
USER golang-profiling

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD /usr/local/bin/golang-profiling --help > /dev/null || exit 1

# Default command
ENTRYPOINT ["/usr/local/bin/golang-profiling"]
CMD ["--help"]

# Labels
LABEL maintainer="golang-profiling team"
LABEL description="High-performance Golang CPU profiler with flame graph generation"
LABEL version="0.1.0"