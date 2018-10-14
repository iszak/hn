FROM golang:1
WORKDIR /go/src/hn/
COPY . .
RUN go get -u github.com/golang/dep/cmd/dep && dep ensure
RUN CGO_ENABLED=0 GOOS=linux go build .

FROM alpine:latest
RUN apk --no-cache add ca-certificates
COPY --from=0 /go/src/hn/ /root/
ENTRYPOINT ["/root/hn"]
