ARG BUILDPLATFORM
ARG TARGETPLATFORM
ARG BASE_IMAGE=alpine:3.24.4@sha256:f59abe656b692d901e184be3445e084786602108599d2a51d229c0f5a48c71bf

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
