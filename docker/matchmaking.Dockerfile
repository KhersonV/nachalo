FROM golang:1.24-alpine

WORKDIR /app/beckend/matchmaking

COPY beckend /app/beckend

RUN go mod download
RUN go build -o /usr/local/bin/nachalo-matchmaking .

EXPOSE 8002

CMD ["/usr/local/bin/nachalo-matchmaking"]
