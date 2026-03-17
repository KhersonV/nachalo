FROM golang:1.24-alpine

WORKDIR /app/beckend/matchmaking

COPY beckend /app/beckend

RUN go mod download

EXPOSE 8002

CMD ["go", "run", "."]