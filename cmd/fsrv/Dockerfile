FROM golang
WORKDIR /go/src/github.com/vbatts/utils/cmd/fsrv
COPY . .
RUN go get -d -v ./...
RUN go install -v ./...
RUN mkdir /data
EXPOSE 8888
CMD ["fsrv", "-root=/data", "-b=0.0.0.0" ]
