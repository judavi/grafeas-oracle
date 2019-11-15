FROM golang:1.12.5
RUN apt-get update && \
    apt-get install -y openjdk-8-jdk
COPY . /go/src/github.com/judavi/grafeas-oracle/
WORKDIR /go/src/github.com/judavi/grafeas-oracle
RUN make build 
WORKDIR /go/src/github.com/judavi/grafeas-oracle/go/v1beta1/main
RUN GO111MODULE=on CGO_ENABLED=0 go build -o grafeas-server .

FROM alpine:latest
WORKDIR /
COPY --from=0 /go/src/github.com/judavi/grafeas-oracle/go/v1beta1/main/grafeas-server /grafeas-server
EXPOSE 8080
ENTRYPOINT ["/grafeas-server"]