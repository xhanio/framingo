FROM ubuntu:22.04

# Install dependencies
RUN apt-get update && apt-get install -y \
    ca-certificates \
    tzdata \
    && rm -rf /var/lib/apt/lists/*

# Create app directory and log directory
RUN mkdir -p /app /var/log/framingo-example /etc/framingo-example

# Set working directory
WORKDIR /app

# Copy the binary from the build context
COPY example-app /app/example-app

# Copy configuration
COPY config.yaml /etc/framingo-example/config.yaml

# Expose API port
EXPOSE 8080

# Expose pprof port (optional, for debugging)
EXPOSE 6060

# Set the entrypoint
ENTRYPOINT ["/app/example-app"]

# Default command
CMD ["daemon", "-c", "/etc/framingo-example/config.yaml"]
