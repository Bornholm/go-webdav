FROM alpine:3.23

ARG TARGETPLATFORM
ENTRYPOINT ["/usr/bin/server"]

RUN apk add gcompat ca-certificates && update-ca-certificates

COPY $TARGETPLATFORM/server /usr/bin/
COPY ./misc/release/config.json /etc/go-webdav/config.json

VOLUME /data

CMD ["-config", "/etc/go-webdav/config.json"]