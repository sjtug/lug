FROM golang:1.9
MAINTAINER Zheng Luo <vicluo96@gmail.com>
RUN apt-get update && apt-get install rsync -y
WORKDIR /go/src/github.com/sjtug/lug
COPY . .
RUN curl https://glide.sh/get | sh
RUN glide install
CMD ["go-wrapper", "run"] # ["app"]
