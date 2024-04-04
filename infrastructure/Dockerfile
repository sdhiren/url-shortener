FROM golang:alpine

WORKDIR /app
COPY . /app

RUN go build -o ./main ./urlshortener

EXPOSE 8080

CMD ["./main"]