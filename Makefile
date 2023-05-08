export CGO_CFLAGS_ALLOW=-D__BLST_PORTABLE__
export CGO_CFLAGS=-D__BLST_PORTABLE__

all: build
.PHONY: all

## variables

# git modules that need to be loaded
MODULES:=

ldflags=-X=github.com/filecoin-project/venus-market/v2/version.CurrentCommit=+git.$(subst -,.,$(shell git describe --always --match=NeVeRmAtCh --dirty 2>/dev/null || git rev-parse --short HEAD 2>/dev/null))
ifneq ($(strip $(LDFLAGS)),)
	    ldflags+=-extldflags=$(LDFLAGS)
	endif

GOFLAGS+=-ldflags="$(ldflags)"

## FFI

FFI_PATH:=extern/filecoin-ffi/
FFI_DEPS:=.install-filcrypto
FFI_DEPS:=$(addprefix $(FFI_PATH),$(FFI_DEPS))

$(FFI_DEPS): build-dep/.filecoin-install ;

build-dep/.filecoin-install: $(FFI_PATH)
	$(MAKE) -C $(FFI_PATH) $(FFI_DEPS:$(FFI_PATH)%=%)
	@touch $@

MODULES+=$(FFI_PATH)
BUILD_DEPS+=build-dep/.filecoin-install
CLEAN+=build-dep/.filecoin-install

## modules
build-dep:
	mkdir $@

$(MODULES): build-dep/.update-modules;
# dummy file that marks the last time modules were updated
build-dep/.update-modules: build-dep;
	git submodule update --init --recursive
	touch $@

## build

test:
	go test -race ./...

lint: $(BUILD_DEPS)
	golangci-lint run

deps: $(BUILD_DEPS)

dist-clean:
	git clean -xdff
	git submodule deinit --all -f

build: $(BUILD_DEPS)
	rm -f market-client
	rm -f venus-market
	go build -o ./market-client $(GOFLAGS) ./cmd/market-client
	go build -o ./venus-market $(GOFLAGS) ./cmd/venus-market

add-debug-flag:
GOFLAGS+=-gcflags="all=-N -l"

debug: add-debug-flag build

# docker
.PHONY: docker

TAG:=test
docker: $(BUILD_DEPS)
ifdef DOCKERFILE
	cp $(DOCKERFILE) .
else
	curl -O https://raw.githubusercontent.com/filecoin-project/venus-docs/master/script/docker/dockerfile
endif
	docker build --build-arg HTTPS_PROXY=$(BUILD_DOCKER_PROXY) --build-arg BUILD_TARGET=venus-market -t venus-market .
	docker build --build-arg HTTPS_PROXY=$(BUILD_DOCKER_PROXY) --build-arg BUILD_TARGET=market-client -t market-client .
	docker tag venus-market filvenus/venus-market:$(TAG)
	docker tag market-client filvenus/market-client:$(TAG)
ifdef PRIVATE_REGISTRY
	docker tag venus-market $(PRIVATE_REGISTRY)/filvenus/venus-market:$(TAG)
	docker tag market-client $(PRIVATE_REGISTRY)/filvenus/market-client:$(TAG)
endif



docker-push: docker
	docker push $(PRIVATE_REGISTRY)/filvenus/venus-market:$(TAG)
	docker push $(PRIVATE_REGISTRY)/filvenus/market-client:$(TAG)
