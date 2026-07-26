FROM golang:1.26-alpine AS builder
WORKDIR /build
COPY go.work go.work.sum* ./
COPY go.mod go.sum* ./
COPY api/go.mod api/go.sum* ./api/
COPY judge/go.mod judge/go.sum* ./judge/
COPY pkg/go.mod ./pkg/
RUN cd api && go mod download
COPY . .
RUN cd api && CGO_ENABLED=0 GOOS=linux go build \
    -trimpath -ldflags="-s -w" \
    -o /bin/api ./cmd/server

FROM alpine:3.21
RUN apk add --no-cache ca-certificates tzdata \
    && addgroup -S ascend && adduser -S -G ascend ascend
COPY --from=builder /bin/api /bin/api
COPY --from=builder /build/migrations /migrations
ENV MIGRATIONS_PATH=/migrations
USER ascend
EXPOSE 8080
HEALTHCHECK --interval=15s --timeout=3s --start-period=5s --retries=3 \
    CMD wget -qO- http://127.0.0.1:8080/healthz || exit 1
ENTRYPOINT ["/bin/api"]
