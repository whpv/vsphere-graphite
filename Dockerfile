FROM alpine

RUN set -ex && \
    apk update && \
    apk add ca-certificates git go libc-dev && \
    mkdir -p /go/src /go/bin && chmod -R 777 /go && \
    export GOPATH=/go && go get github.com/whpv/vsphere-graphite && strip /go/bin/vsphere-graphite && \
    apk del git go libc-dev

ADD vsphere-graphite.json /etc/vsphere-graphite.json
ENV PATH /go/bin:$PATH

CMD ["vsphere-graphite"]