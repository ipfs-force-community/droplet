export CGO_CFLAGS_ALLOW=-D__BLST_PORTABLE__
export CGO_CFLAGS=-D__BLST_PORTABLE__

all: build
.PHONY: all

## variables

# git modules that need to be loaded
MODULES:=

ldflags=-X=github.com/ipfs-force-community/droplet/v2/version.CurrentCommit=+git.$(subst -,.,$(shell git describe --always --match=NeVeRmAtCh --dirty 2>/dev/null || git rev-parse --short HEAD 2>/dev/null))
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
	rm -f droplet-client
	rm -f droplet
	go build -o ./droplet-client $(GOFLAGS) ./cmd/droplet-client
	go build -o ./droplet $(GOFLAGS) ./cmd/droplet

droplet: $(BUILD_DEPS)
	rm -f droplet
	go build -o ./droplet $(GOFLAGS) ./cmd/droplet

droplet-client: $(BUILD_DEPS)
	rm -f droplet-client
	go build -o ./droplet-client $(GOFLAGS) ./cmd/droplet-client

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
	docker build --build-arg HTTPS_PROXY=$(BUILD_DOCKER_PROXY) --build-arg BUILD_TARGET=droplet -t droplet .
	docker build --build-arg HTTPS_PROXY=$(BUILD_DOCKER_PROXY) --build-arg BUILD_TARGET=droplet-client -t droplet-client .
	docker tag droplet filvenus/droplet:$(TAG)
	docker tag droplet-client filvenus/droplet-client:$(TAG)
ifdef PRIVATE_REGISTRY
	docker tag droplet $(PRIVATE_REGISTRY)/filvenus/droplet:$(TAG)
	docker tag droplet-client $(PRIVATE_REGISTRY)/filvenus/droplet-client:$(TAG)
endif



docker-push: docker
ifdef PRIVATE_REGISTRY
	docker push $(PRIVATE_REGISTRY)/filvenus/droplet:$(TAG)
	docker push $(PRIVATE_REGISTRY)/filvenus/droplet-client:$(TAG)
else
	docker tag droplet filvenus/droplet:latest
	docker tag droplet-client filvenus/droplet-client:latest
	docker push filvenus/droplet:latest
	docker push filvenus/droplet-client:latest
	docker push filvenus/droplet:$(TAG)
	docker push filvenus/droplet-client:$(TAG)
endif
