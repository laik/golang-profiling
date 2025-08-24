# Runtime stage only - binary will be built locally and copied
FROM ubuntu:latest

# 使用中科大镜像源加速
RUN sed -i 's/archive.ubuntu.com/mirrors.ustc.edu.cn/g' /etc/apt/sources.list && \
    sed -i 's/security.ubuntu.com/mirrors.ustc.edu.cn/g' /etc/apt/sources.list

# Install runtime dependencies including Perl
RUN apt-get update && apt-get install -y \
    ca-certificates \
    perl \
    util-linux \
    && rm -rf /var/lib/apt/lists/*

# Copy the pre-built binary from local build
COPY golang-profiling-bin /usr/local/bin/golang-profiling

# Copy flamegraph.pl script
COPY flamegraph.pl /usr/local/bin/flamegraph.pl

# Copy crictl from local system
COPY crictl /usr/local/bin/crictl

# Make them executable
RUN chmod +x /usr/local/bin/golang-profiling && \
    chmod +x /usr/local/bin/flamegraph.pl && \
    chmod +x /usr/local/bin/crictl

# Create non-root user
RUN groupadd -g 1001 rustuser && \
    useradd -r -u 1001 -g rustuser -d /var/empty -s /sbin/nologin rustuser

USER rustuser

EXPOSE 8080

CMD ["golang-profiling"]