FROM debian
MAINTAINER Zheng Luo <vicluo96@gmail.com>
RUN apt-get update && apt-get install rsync -y
COPY ./lug /lug
WORKDIR /
ENTRYPOINT "/lug"
