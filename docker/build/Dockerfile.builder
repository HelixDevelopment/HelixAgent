# HelixAgent Release Builder Container
# Used by scripts/build/build-release.sh for reproducible release builds.
FROM docker.io/golang:1.26-alpine

RUN apk update && apk add --no-cache git bash coreutils jq make ca-certificates || \
    (sleep 5 && apk update && apk add --no-cache git bash coreutils jq make ca-certificates)

WORKDIR /build

COPY scripts/build/build-container.sh /build/scripts/build/build-container.sh
RUN chmod +x /build/scripts/build/build-container.sh

ENTRYPOINT ["/build/scripts/build/build-container.sh"]
