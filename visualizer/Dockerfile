FROM golang:1.20-buster as builder
WORKDIR /app

COPY go.* ./
RUN go mod download

COPY ./app ./

RUN go build -o server

FROM debian:buster-slim
RUN set -x && apt-get update && DEBIAN_FRONTEND=noninteractive apt-get install -y \
    ca-certificates && \
    rm -rf /var/lib/apt/lists/*

RUN mkdir -p /keys

# Copy the binary to the production image from the builder stage.
COPY --from=builder /app/server /server
COPY ./instances.html instances.html
# Run the web service on container startup.
CMD ["/server"]
