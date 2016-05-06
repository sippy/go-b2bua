GOPATH=/usr/local/share/go
all: b2bua_simple

b2bua_simple: *.go src/sippy/*.go src/sippy/conf/*.go src/sippy/time/*.go src/sippy/log/*.go src/sippy/utils/*.go src/sippy/headers/*.go src/sippy/types/*.go src/sippy/sdp/*.go
	GOPATH=$(GOPATH):$(CURDIR) go build -o b2bua_simple

clean:
	-rm b2bua_simple
