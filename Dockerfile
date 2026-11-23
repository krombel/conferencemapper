# builder
FROM golang:1.27.3-alpine@sha256:fa3d596898d9f126af7c01d14ee7597bab3f39cccf392ef36b69fe77cd681f18 AS builder
RUN apk add --no-cache gcc musl-dev
WORKDIR /app
ENV CGO_ENABLED=1
COPY ./go.mod ./
COPY ./go.sum ./
RUN go mod download
RUN go mod verify
COPY ./main.go ./main.go
RUN go build -o main .

# final image
FROM alpine:3.24.3@sha256:be2656061b536c1883c4034790e21d0a09c5aaaf51968b89368a3c0371ea2c9a

LABEL org.opencontainers.image.source=https://github.com/krombel/conferencemapper
LABEL org.opencontainers.image.description="Jitsi Conferencemapper"
LABEL org.opencontainers.image.licenses=Apache-2.0

RUN mkdir -p /app/data
WORKDIR /app
COPY --from=builder /app/main .

EXPOSE 8001
CMD ["/app/main", "-dbPath", "/app/data/conferencemapper.db"]