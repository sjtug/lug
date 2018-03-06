FROM golang:1.9
MAINTAINER Zheng Luo <vicluo96@gmail.com>
RUN apt-get update && apt-get install rsync -y
WORKDIR /go/src/github.com/sjtug/lug
COPY . .
RUN curl https://raw.githubusercontent.com/golang/dep/master/install.sh | sh
RUN dep ensure
RUN go-wrapper install
CMD ["go-wrapper", "run"] # ["app"]
