# syntax=docker/dockerfile:1
FROM golang:alpine AS builder

WORKDIR /app

ENV GOTOOLCHAIN=auto

RUN apk add --no-cache ca-certificates git

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /bin/datamask ./cmd/datamask

FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata

COPY --from=builder /bin/datamask /usr/local/bin/datamask

ENTRYPOINT ["/usr/local/bin/datamask"]
CMD ["--help"]
