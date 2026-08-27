FROM ubuntu:24.04
RUN apt-get update \
 && apt-get install -y --no-install-recommends curl ca-certificates \
 && rm -rf /var/lib/apt/lists/*
RUN curl -sSL https://nixpacks.com/install.sh | bash
ENTRYPOINT ["nixpacks"]
