ARG BASE_IMAGE=alpine:3.22.1@sha256:4bcff63911fcb4448bd4fdacec207030997caf25e9bea4045fa6c8c44de311d1
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
