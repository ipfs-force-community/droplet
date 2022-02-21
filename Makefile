export CGO_CFLAGS_ALLOW=-D__BLST_PORTABLE__
export CGO_CFLAGS=-D__BLST_PORTABLE__

all: build
.PHONY: all

FFI_PATH:=extern/filecoin-ffi/
FFI_DEPS:=.install-filcrypto
FFI_DEPS:=$(addprefix $(FFI_PATH),$(FFI_DEPS))

$(FFI_DEPS): build-dep/.filecoin-install ;
MODULES:=

CLEAN:=

ldflags=-X=github.com/filecoin-project/venus-market/version/build.CurrentCommit=+git.$(subst -,.,$(shell git describe --always --match=NeVeRmAtCh --dirty 2>/dev/null || git rev-parse --short HEAD 2>/dev/null))
ifneq ($(strip $(LDFLAGS)),)
	    ldflags+=-extldflags=$(LDFLAGS)
	endif

GOFLAGS+=-ldflags="$(ldflags)"

build-dep:
	mkdir $@

build-dep/.filecoin-install: $(FFI_PATH) build-dep
	    $(MAKE) -C $(FFI_PATH) $(FFI_DEPS:$(FFI_PATH)%=%)
		    @touch $@

MODULES+=$(FFI_PATH)
BUILD_DEPS+=build-dep/.filecoin-install
CLEAN+=build-dep/.filecoin-install

$(MODULES): build-dep/.update-modules ;

# dummy file that marks the last time modules were updated
build-dep/.update-modules: build-dep
	git submodule update --init --recursive
	touch $@

CLEAN+=build-dep/.update-modules

test:
	rm -rf models/test_sqlite_db*
	go test -race ./...

lint: $(BUILD_DEPS)
	go run github.com/golangci/golangci-lint/cmd/golangci-lint run

dist-clean:
	git clean -xdff
	git submodule deinit --all -f

build: $(BUILD_DEPS)
	rm -f chain-co
	go build -o ./market-client $(GOFLAGS) ./cmd/market-client
	go build -o ./market-client $(GOFLAGS) ./cmd/market-client