ARG BASE_IMAGE=alpine:3.23.3@sha256:eb37f58646a901dc7727cf448cae36daaefaba79de33b5058dab79aa4c04aefb
ARG GOOS=linux
ARG GOARCH

FROM --platform=$BUILDPLATFORM $BASE_IMAGE AS builder

RUN apk add --no-cache \
    ca-certificates \
    tzdata

FROM scratch

ARG BASE_IMAGE

LABEL "org.opencontainers.image.url"="https://shoutrrr.nickfedor.com/" \
    "org.opencontainers.image.documentation"="https://shoutrrr.nickfedor.com/" \
    "org.opencontainers.image.source"="https://github.com/nicholas-fedor/shoutrrr" \
    "org.opencontainers.image.licenses"="MIT" \
    "org.opencontainers.image.title"="Shoutrrr" \
    "org.opencontainers.image.description"="A notification library for gophers and their furry friends." \
    "org.opencontainers.image.base.name"="${BASE_IMAGE}"

# Copy ca-certs and timezone from builder
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo

# Copy binary (GoReleaser places the platform-specific binary at the context root as 'shoutrrr')
COPY shoutrrr /

ENTRYPOINT ["/shoutrrr"]
