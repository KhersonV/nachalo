FROM golang:1.24-alpine

WORKDIR /app/beckend/gameservice

COPY beckend /app/beckend

RUN go mod download
RUN go build -o /usr/local/bin/nachalo-gameservice ./cmd

EXPOSE 8001

CMD ["/usr/local/bin/nachalo-gameservice"]
