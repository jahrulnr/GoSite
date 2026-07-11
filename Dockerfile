FROM node:20-bookworm AS webbuilder

WORKDIR /src
COPY web/package.json web/package-lock.json ./web/
RUN cd web && npm install
COPY web/ ./web/
RUN cd web && npm run build

FROM golang:1.26.4-bookworm AS gobuilder
ARG VERSION=dev

WORKDIR /src
RUN apt-get update \
    && apt-get install -y --no-install-recommends zip \
    && rm -rf /var/lib/apt/lists/*
COPY go.mod go.sum ./
RUN go mod download

COPY . .
COPY --from=webbuilder /src/internal/delivery/http/frontend/dist ./internal/delivery/http/frontend/dist
RUN make bundled-plugins
RUN CGO_ENABLED=0 go build -ldflags "-X github.com/jahrulnr/gosite/internal/buildinfo.Version=${VERSION}" -o /out/gosite ./cmd/gosite

FROM nginx:1.30.2-trixie

SHELL ["/bin/bash", "-c"]

ENV TZ="Asia/Jakarta"
ENV PATH="/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"
ENV STORAGE_PATH="/storage"
ENV DB_DATABASE="/storage/db.sqlite"
ENV WEB_PATH="/www"
ENV TEMPLATES_DIR="/var/setup"
ENV MIGRATIONS_DIR="/app/migrations"
ENV LISTEN_ADDR=":8080"
ENV PLUGIN_BUNDLED_PATH="/app/bundled-plugins"
ENV PLUGIN_BUNDLED_ENABLED="true"
ENV FE_EMBED="true"
ENV TLS_ENABLE="true"

RUN apt-get update \
    && apt-get install -y --no-install-recommends \
        curl \
        ca-certificates \
        tzdata \
        openssl \
        make \
        certbot \
        python3-certbot-nginx \
        fuse3 \
        s3fs \
        zip \
        unzip \
        logrotate \
    && groupadd -g 1000 apps \
    && useradd -u 1000 -g 1000 apps \
    && apt-get clean \
    && rm -rf /var/lib/apt/lists/* /tmp/* /var/tmp/*

COPY docker/nginx-vts/build.sh /tmp/build-nginx-vts.sh
RUN chmod +x /tmp/build-nginx-vts.sh && /tmp/build-nginx-vts.sh && rm -f /tmp/build-nginx-vts.sh

ENV GOSITE_NGINX_VTS_URL="http://127.0.0.1:18082/status/format/json"

RUN mkdir -p /storage /var/setup \
    && rm -f /etc/fstab

COPY --from=gobuilder /out/gosite /usr/local/bin/gosite
COPY --from=gobuilder /src/dist/bundled-plugins /app/bundled-plugins
COPY ./migrations /app/migrations
COPY ./config/nginx /var/setup/nginx
COPY ./config/webconfig /var/setup/webconfig
COPY ./config/fstab_mounter.sh /run/fstab_mounter.sh
COPY ./config/start.sh /run/start.sh
COPY ./config/logrotate/gosite /etc/logrotate.d/gosite

# Carry upstream nginx defaults (mime.types, fastcgi_params) into the staged tree
# so they survive when start.sh moves /var/setup/nginx into /etc/nginx.
RUN cp -a /etc/nginx/mime.types /var/setup/nginx/mime.types \
    && cp -a /etc/nginx/fastcgi_params /var/setup/nginx/fastcgi_params \
    && cp -a /etc/nginx/fastcgi.conf /var/setup/nginx/fastcgi.conf \
    && cp -a /etc/nginx/uwsgi_params /var/setup/nginx/uwsgi_params \
    && cp -a /etc/nginx/scgi_params /var/setup/nginx/scgi_params 2>/dev/null || true \
    && cp -a /etc/nginx/koi-utf /var/setup/nginx/koi-utf 2>/dev/null || true \
    && cp -a /etc/nginx/koi-win /var/setup/nginx/koi-win 2>/dev/null || true \
    && cp -a /etc/nginx/win-utf /var/setup/nginx/win-utf 2>/dev/null || true

RUN chmod +x /usr/local/bin/gosite /run/start.sh /run/fstab_mounter.sh

EXPOSE 80
EXPOSE 443
EXPOSE 443/udp
EXPOSE 8080

WORKDIR /app
CMD ["/run/start.sh"]
HEALTHCHECK --interval=30s --timeout=10s --start-period=30s --retries=3 CMD ["curl", "--fail", "-k", "https://localhost:8080"]
