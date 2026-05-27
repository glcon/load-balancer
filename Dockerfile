FROM golang:1.25-alpine

WORKDIR /app

COPY . .

RUN go build -o loadbalancer .

EXPOSE 8080 9090

# command to run when the container starts
CMD ["./loadbalancer", "--config=/app/config.yml"]