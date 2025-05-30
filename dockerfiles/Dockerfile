FROM alpine:3.22.0@sha256:8a1f59ffb675680d47db6337b49d22281a139e9d709335b492be023728e11715 AS alpine

RUN apk add --no-cache ca-certificates

FROM scratch

COPY --from=alpine \
    /etc/ssl/certs/ca-certificates.crt \
    /etc/ssl/certs/ca-certificates.crt
COPY shoutrrr/shoutrrr /

ENTRYPOINT ["/shoutrrr"]
