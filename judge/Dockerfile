FROM golang:1.26-alpine AS builder
WORKDIR /build
COPY go.work go.work.sum* ./
COPY go.mod go.sum* ./
COPY api/go.mod api/go.sum* ./api/
COPY judge/go.mod judge/go.sum* ./judge/
COPY pkg/go.mod ./pkg/
RUN cd judge && go mod download
COPY . .
RUN cd judge && CGO_ENABLED=0 go build -o /bin/worker ./cmd/worker

FROM alpine:3.21
RUN apk add --no-cache ca-certificates docker-cli
COPY --from=builder /bin/worker /bin/worker
ENTRYPOINT ["/bin/worker"]
