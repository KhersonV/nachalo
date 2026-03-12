FROM golang:1.24-alpine

WORKDIR /app/beckend/gameservice

COPY beckend /app/beckend

RUN go mod download

EXPOSE 8001

CMD ["go", "run", "./cmd"]