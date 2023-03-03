ALL_TARGERS=	b2bua_simple b2bua_radius call_transfer rfc8760 stir_shaken

all: ${ALL_TARGERS}

SIPPY_DEPS=	sippy/*.go sippy/conf/*.go sippy/time/*.go sippy/log/*.go sippy/utils/*.go sippy/headers/*.go sippy/types/*.go sippy/sdp/*.go sippy/security/*.go

b2bua_simple: *.go ${SIPPY_DEPS}
	go build -o b2bua_simple

b2bua_radius: cmd/b2bua_radius/*.go internal/b2bua_radius/*.go ${SIPPY_DEPS}
	go build -o b2bua_radius cmd/b2bua_radius/main.go

call_transfer: examples/call_transfer/*.go internal/call_transfer/*.go ${SIPPY_DEPS}
	go build -o call_transfer examples/call_transfer/main.go

rfc8760: examples/rfc8760/*.go internal/rfc8760/*.go ${SIPPY_DEPS}
	go build -o rfc8760 examples/rfc8760/main.go

stir_shaken: examples/stir_shaken/*.go internal/stir_shaken/*.go ${SIPPY_DEPS}
	go build -o stir_shaken examples/stir_shaken/main.go

clean:
	-rm ${ALL_TARGERS}

test:
	go test ./...
