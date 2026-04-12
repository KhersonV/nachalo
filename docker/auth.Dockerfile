FROM golang:1.24-alpine

WORKDIR /app/beckend/auth

COPY beckend /app/beckend

RUN go mod download
RUN go build -o /usr/local/bin/nachalo-auth Auth.go

EXPOSE 8000

CMD ["/usr/local/bin/nachalo-auth"]
