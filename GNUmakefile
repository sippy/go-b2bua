GOPATH=/usr/local/share/go
all: b2bua_simple

b2bua_simple: *.go src/sippy/*.go src/sippy/conf/*.go src/sippy/time/*.go src/sippy/log/*.go src/sippy/utils/*.go src/sippy/headers/*.go src/sippy/types/*.go src/sippy/sdp/*.go src/sippy/security/*.go
	GOPATH=$(CURDIR):$(GOPATH) GO111MODULE=off go build -o b2bua_simple

clean:
	-rm b2bua_simple

test:
	cd $(CURDIR)/src/sippy; GOPATH=$(CURDIR):$(GOPATH) GO111MODULE=off go test
