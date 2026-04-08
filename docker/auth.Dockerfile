FROM golang:1.24-alpine

WORKDIR /app/beckend/auth

COPY beckend /app/beckend

RUN go mod download

EXPOSE 8000

CMD ["go", "run", "Auth.go"]