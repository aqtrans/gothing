FROM aqtrans/golang-npm:latest

RUN mkdir -p /go/src/thing
WORKDIR /go/src/thing

## Running with PWD: docker run -it --rm --name gowiki-instance -p 3000:3000 -v (PWD):/go/src/wiki -w /go/src/wiki gowiki
## Running: docker run -it --rm --name gowiki-instance -p 3000:3000 -w /go/src/wiki gowiki

ADD . /go/src/thing/
RUN /bin/sh ./build_css.sh
RUN go get github.com/kardianos/govendor && govendor sync
RUN go get -d
RUN go build -o ./thing

# Set the entry point of the container to the bee command that runs the
# application and watches for changes
CMD ["./thing"]
