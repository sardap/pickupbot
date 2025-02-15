FROM golang:latest as builder

WORKDIR /app
COPY go.mod .
COPY go.sum .
RUN go mod download
COPY . .
RUN go build -o main .

# Backend
FROM jrottenberg/ffmpeg:latest

ENV DB_PATH=data/db.bin

WORKDIR /app

RUN mkdir data

COPY --from=builder /app/main main

ENTRYPOINT [ "/app/main" ]
