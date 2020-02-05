FROM golang:1.12.0-stretch as builder

COPY . /randomGopher

WORKDIR /randomGopher

ENV GO111MODULE=on

RUN CGO_ENABLED=0 GOOS=linux go build -o RandomGopher

#second stage
FROM alpine:latest

WORKDIR /root/

COPY --from=builder /randomGopher .

CMD ["./RandomGopher"]