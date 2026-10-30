FROM golang:1.25.5

WORKDIR /usr/src/app

COPY ./src .
RUN go mod tidy
RUN go mod download

COPY ./quizzes /quizzes
EXPOSE 5656
RUN go build -v -o /usr/local/bin/app ./...

CMD ["app"]
