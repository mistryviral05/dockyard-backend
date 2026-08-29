FROM ubuntu:24.04

ARG NIXPACKS_VERSION=1.41.0

RUN apt-get update \
    && apt-get install -y --no-install-recommends \
        curl \
        ca-certificates \
    && rm -rf /var/lib/apt/lists/*

RUN curl -fL \
    "https://github.com/railwayapp/nixpacks/releases/download/v${NIXPACKS_VERSION}/nixpacks-v${NIXPACKS_VERSION}-amd64.deb" \
    -o /tmp/nixpacks.deb \
    && dpkg -i /tmp/nixpacks.deb \
    && rm /tmp/nixpacks.deb

RUN nixpacks --version

ENTRYPOINT ["nixpacks"]
