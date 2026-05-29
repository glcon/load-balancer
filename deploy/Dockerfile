FROM golang:1.25-alpine

WORKDIR /app

COPY . .

# Build the load balancer binary from the `cmd/lb` entrypoint
RUN go build -o loadbalancer ./cmd/lb

EXPOSE 8080 9090

# command to run when the container starts
CMD ["./loadbalancer", "--config=/app/config.yml"]