FROM golang:1.10 AS build-env
# The GOPATH in the image is /go.
ADD . /go/src/github.com/sjtug/lug
WORKDIR /go/src/github.com/sjtug/lug
RUN curl https://raw.githubusercontent.com/golang/dep/master/install.sh | sh
RUN dep ensure
RUN go build github.com/sjtug/lug/cli/lug

FROM debian:9
WORKDIR /app
COPY --from=build-env /go/src/github.com/sjtug/lug/lug /app/
ENTRYPOINT ["./lug"]
