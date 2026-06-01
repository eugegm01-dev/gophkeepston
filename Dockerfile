FROM golang:1.25-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o /gophkeepston ./cmd/server

FROM alpine:3.19
RUN apk add --no-cache ca-certificates
COPY --from=builder /gophkeepston /gophkeepston
EXPOSE 50051
ENTRYPOINT ["/gophkeepston"]