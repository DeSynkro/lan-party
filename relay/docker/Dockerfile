# Runtime image for the self-hosted LAN Party relay.
#
# Build from a directory that contains the static lan-party-relay binary:
#   docker build -t lan-party-relay:local -f docker/Dockerfile .
#
# The binary is built with CGO_ENABLED=0, so the runtime image needs no libc,
# shell or package manager.
FROM busybox:1.36 AS data
RUN mkdir -p /data && touch /data/.keep

FROM gcr.io/distroless/static-debian12:nonroot
ARG VERSION=dev
LABEL org.opencontainers.image.title="LAN Party Relay"
LABEL org.opencontainers.image.description="Opaque UDP relay for LAN Party host and friend connections"
LABEL org.opencontainers.image.version="${VERSION}"
LABEL org.opencontainers.image.licenses="GPL-3.0-only"
LABEL org.opencontainers.image.source="https://github.com/DeSynkro/lan-party"
LABEL org.opencontainers.image.url="https://github.com/DeSynkro/lan-party"

COPY --from=data --chown=65532:65532 /data /data
COPY --chown=65532:65532 lan-party-relay /lan-party-relay

WORKDIR /data
USER 65532:65532
EXPOSE 7443/tcp
EXPOSE 7443/udp
ENTRYPOINT ["/lan-party-relay"]
CMD ["run"]
