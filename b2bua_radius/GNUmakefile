GOPATH=/usr/local/share/go

all: b2bua_radius

b2bua_radius: *.go
	GOPATH=$(GOPATH):$(CURDIR)/../ go build -o b2bua_radius

clean:
	-rm b2bua_radius
