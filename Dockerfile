FROM golang

RUN mkdir -p /go/src/github.com/davidmontoyago/di-terraform-repo-pull-controller

ADD . /go/src/github.com/davidmontoyago/di-terraform-repo-pull-controller

WORKDIR /go

RUN go get ./...
RUN go install -v ./...

CMD ["/go/bin/di-terraform-repo-pull-controller"]
