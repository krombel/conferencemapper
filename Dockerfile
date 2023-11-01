# builder
FROM golang:1.21.3-alpine AS builder
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
FROM alpine:3.18.4

LABEL org.opencontainers.image.source=https://github.com/krombel/conferencemapper
LABEL org.opencontainers.image.description="Jitsi Conferencemapper"
LABEL org.opencontainers.image.licenses=Apache-2.0

RUN mkdir -p /app/data
WORKDIR /app
COPY --from=builder /app/main .

EXPOSE 8001
CMD ["/app/main", "-dbPath", "/app/data/conferencemapper.db"]