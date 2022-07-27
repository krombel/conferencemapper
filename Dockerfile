# builder
FROM golang:1.18.4-alpine AS builder
RUN apk --no-cache add gcc musl-dev
RUN mkdir /app
ADD . /app
WORKDIR /app

RUN go mod download
RUN go build -o main .

# final image
FROM alpine
RUN mkdir -p /app/data
WORKDIR /app
COPY --from=builder /app/main .

EXPOSE 8001
CMD ["/app/main", "-dbPath", "/app/data/conferencemapper.db"]