FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -o /out/sub-mixer ./cmd/sub-mixer && \
    CGO_ENABLED=0 go build -trimpath -o /out/extras-api ./cmd/extras-api

FROM alpine:3.20
LABEL org.opencontainers.image.source="https://github.com/egor-muindor/submix" \
      org.opencontainers.image.description="Extension for the Remnawave 3.x panel: mixes external and static connection links into subscriptions" \
      org.opencontainers.image.licenses="GPL-3.0-or-later"
RUN apk add --no-cache ca-certificates && adduser -D -u 10001 submix
COPY --from=build /out/sub-mixer /usr/local/bin/sub-mixer
COPY --from=build /out/extras-api /usr/local/bin/extras-api
USER submix
ENTRYPOINT ["/usr/local/bin/sub-mixer"]
