# Built by `gopro build image -e <env>`, which runs `docker build` with the
# project root as the context and passes BASE (build.images[].base), NAME and
# CONFIG_TGT - the env's rendered config directory. So for an env, first:
#   gopro build binary -e <env>
#   gopro generate config -e <env>
ARG BASE=ubuntu:22.04
FROM ${BASE}

ARG NAME=exampleapp
ARG CONFIG_TGT=dist/local/config

# Install dependencies
RUN apt-get update && apt-get install -y \
    ca-certificates \
    tzdata \
    && rm -rf /var/lib/apt/lists/*

# Create app directory and log directory
RUN mkdir -p /app /var/log/framingo-example /etc/framingo-example

# Set working directory
WORKDIR /app

# The binary gopro built for this env, with its build metadata - the product
# name among it, which is the prefix of every environment override
COPY bin/${NAME} /app/exampleapp

# The env's rendered config; on Kubernetes the ConfigMap is mounted over it
COPY ${CONFIG_TGT}/${NAME}/config.yaml /etc/framingo-example/config.yaml

# The schema migrations, for `exampleapp migrate` (the prod Job), or for a
# daemon whose db.migration.dir points here (docker compose)
COPY ${CONFIG_TGT}/${NAME}/migrations /app/migrations

# Expose API port
EXPOSE 8080

# Expose the health listener: /healthz and /readyz for probes
EXPOSE 8081

# Expose pprof port (optional, for debugging)
EXPOSE 6060

# Set the entrypoint
ENTRYPOINT ["/app/exampleapp"]

# Default command
CMD ["daemon", "-c", "/etc/framingo-example/config.yaml"]
