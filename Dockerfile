FROM golang:1.26.4-bookworm AS gobuilder

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -o /out/gosite ./cmd/gosite

FROM nginx:1.30.2-trixie

SHELL ["/bin/bash", "-c"]

ENV TZ="Asia/Jakarta"
ENV STORAGE_PATH="/storage"
ENV DB_DATABASE="/storage/db.sqlite"
ENV WEB_PATH="/www"
ENV TEMPLATES_DIR="/var/setup"
ENV MIGRATIONS_DIR="/app/migrations"
ENV LISTEN_ADDR=":8080"
ENV FE_EMBED="true"
ENV TLS_ENABLE="true"

RUN apt-get update \
    && apt-get install -y --no-install-recommends \
        curl \
        ca-certificates \
        tzdata \
        openssl \
        make \
        supervisor \
        certbot \
        python3-certbot-nginx \
    && groupadd -g 1000 apps \
    && useradd -u 1000 -g 1000 apps \
    && apt-get clean \
    && rm -rf /var/lib/apt/lists/* /tmp/* /var/tmp/*

RUN mkdir -p /storage /var/setup \
    && rm -f /etc/fstab

COPY --from=gobuilder /out/gosite /usr/local/bin/gosite
COPY ./migrations /app/migrations
COPY ./config/nginx /var/setup/nginx
COPY ./config/webconfig /var/setup/webconfig
COPY ./config/supervisord.conf /etc/supervisord.conf
COPY ./config/fstab_mounter.sh /run/fstab_mounter.sh
COPY ./config/start.sh /run/start.sh

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
HEALTHCHECK --interval=30s --timeout=30s --start-period=5s --retries=3 CMD ["curl", "--fail", "http://localhost:80"]
