ALL_TARGERS=	b2bua_simple b2bua_radius call_transfer rfc8760 stir_shaken

all: ${ALL_TARGERS}

SIPPY_DEPS=	sippy/*.go sippy/conf/*.go sippy/time/*.go sippy/log/*.go sippy/utils/*.go sippy/headers/*.go sippy/types/*.go sippy/sdp/*.go sippy/security/*.go

b2bua_simple: cmd/b2bua_simple/*.go ${SIPPY_DEPS}
	go build b2bua_simple

b2bua_radius: cmd/b2bua_radius/*.go ${SIPPY_DEPS}
	go build b2bua_radius

call_transfer: examples/call_transfer/*.go ${SIPPY_DEPS}
	go build call_transfer

rfc8760: examples/rfc8760/*.go ${SIPPY_DEPS}
	go build rfc8760

stir_shaken: examples/stir_shaken/*.go ${SIPPY_DEPS}
	go build stir_shaken

clean:
	-rm ${ALL_TARGERS}

test:
	go test ./sippy
	go test ./cmd/b2bua_radius
