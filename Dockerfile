FROM golang:1.25-alpine AS builder

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /logparser .

FROM alpine:3.21

WORKDIR /app
COPY --from=builder /logparser .
COPY static/ ./static/

EXPOSE 8080
ENTRYPOINT ["./logparser"]
