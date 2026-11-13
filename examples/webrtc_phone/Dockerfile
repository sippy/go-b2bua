# syntax=docker/dockerfile:1
ARG RTPPROXY_IMAGE=sippylabs/rtpproxy:latest
FROM node:22-bookworm-slim AS web
WORKDIR /assets
RUN npm pack sip.js@0.21.2 && mkdir vendor && tar -xzf sip.js-0.21.2.tgz --strip-components=1 -C vendor

FROM golang:1.25-bookworm AS build
WORKDIR /src
COPY . .
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build \
    GOMAXPROCS=8 go build -p 8 -o /out/go-webrtc-phone ./cmd/webrtc_phone

FROM ${RTPPROXY_IMAGE}
USER root
WORKDIR /opt/go-webrtc-phone
COPY --from=build /out/go-webrtc-phone /usr/local/bin/go-webrtc-phone
COPY --from=web /assets/vendor web/vendor
COPY examples/webrtc_phone/web/ web/
COPY examples/webrtc_phone/run.sh ./run.sh
RUN mkdir -p /opt/go-webrtc-phone/tls && \
    openssl req -x509 -newkey rsa:2048 -nodes -days 365 \
      -subj /CN=localhost -addext 'subjectAltName=DNS:localhost,IP:127.0.0.1' \
      -keyout tls/server.key -out tls/server.crt && chmod 600 tls/server.key && chmod +x run.sh
ENV HTTPS_ADDRESS=0.0.0.0:18443 WSS_ADDRESS=0.0.0.0:19876 \
    SIP_ADDRESS=127.0.0.1:15060 OUTBOUND_PROXY=127.0.0.1:15070 \
    RTP_ADDRESS=127.0.0.1 MIN_RTP_PORT=35000 MAX_RTP_PORT=35100
EXPOSE 18443/tcp 19876/tcp 15060/udp 35000-35100/udp
ENTRYPOINT ["./run.sh"]
