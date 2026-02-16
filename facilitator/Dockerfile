FROM ghcr.io/x402-rs/x402-facilitator:latest

RUN apt-get update && apt-get install -y --no-install-recommends wget && rm -rf /var/lib/apt/lists/*

COPY config.json /app/config.json
COPY entrypoint.sh /app/entrypoint.sh

EXPOSE 8402

ENTRYPOINT ["sh", "/app/entrypoint.sh"]
