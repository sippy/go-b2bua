GOPATH=/usr/local/share/go

all: b2bua

b2bua: *.go
	GOPATH=$(CURDIR)/../../:$(GOPATH) GO111MODULE=111 go build -o b2bua

clean:
	-rm b2bua
