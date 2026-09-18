ARG BUILDPLATFORM
ARG TARGETPLATFORM
ARG BASE_IMAGE=alpine:3.24.2@sha256:31b6477333eb8257db9e5d7c3a7264fd0467928756f0bbcc27d35bea5d28cdbd

# Builder stage
FROM --platform=$BUILDPLATFORM ${BASE_IMAGE} AS builder
RUN apk add --no-cache ca-certificates tzdata

# Final build
FROM scratch

ARG TARGETPLATFORM

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo

COPY --chmod=0755 ${TARGETPLATFORM}/shoutrrr /shoutrrr

USER 65534:65534

ENTRYPOINT ["/shoutrrr"]
