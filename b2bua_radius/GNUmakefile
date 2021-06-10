GOPATH=/usr/local/share/go

all: b2bua_radius

b2bua_radius: *.go
	GOPATH=$(CURDIR)/../:$(GOPATH) GO111MODULE=off go build -o b2bua_radius

clean:
	-rm b2bua_radius
