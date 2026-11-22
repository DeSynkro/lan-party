# LAN Party web host.
#
# Build from a checkout of the repository, from this directory:
#   docker compose build
#   docker compose up -d
#
# The image is static and runs as a non-root user. Compose overrides the UID
# and GID to PUID/PGID so the bind-mounted ./data directory is writable.
FROM golang:1.27-bookworm AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY internal ./internal
COPY server ./server

ARG VERSION=dev
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath \
    -ldflags "-s -w -X github.com/desynkro/lan-party/internal/buildinfo.Version=${VERSION}" \
    -o /out/lan-party-web ./server/cmd/lan-party-web

FROM gcr.io/distroless/static-debian12:nonroot
ARG VERSION=dev
LABEL org.opencontainers.image.title="LAN Party Web Host"
LABEL org.opencontainers.image.description="Browser control for a headless LAN Party host"
LABEL org.opencontainers.image.version="${VERSION}"
LABEL org.opencontainers.image.licenses="GPL-3.0-only"
LABEL org.opencontainers.image.source="https://github.com/DeSynkro/lan-party"
LABEL org.opencontainers.image.url="https://github.com/DeSynkro/lan-party"

COPY --from=build --chown=65532:65532 /out/lan-party-web /lan-party-web

USER 65532:65532
EXPOSE 8080/tcp
EXPOSE 51820/udp
ENTRYPOINT ["/lan-party-web"]
CMD ["run", "--data-dir", "/data", "--listen", "0.0.0.0:8080"]
