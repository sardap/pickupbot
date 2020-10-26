FROM golang:latest as builder

WORKDIR /app
COPY go.mod .
COPY go.sum .
RUN go mod download
COPY . .
RUN go build -o main .

# Backend
FROM jrottenberg/ffmpeg:latest

WORKDIR /app
COPY --from=builder /app/main main

ENTRYPOINT [ "/app/main" ]
