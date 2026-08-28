LXP_SCAN_DIR ?= ../lxp-scan
GOBIN := $(shell go env GOPATH)/bin
UNAME := $(shell uname -s)
ifeq ($(UNAME),Darwin)
  DYLIB := liblxp_scan.dylib
else
  DYLIB := liblxp_scan.so
endif

.PHONY: all lib proto build run clean

all: lib proto build

# Build the Rust cdylib and copy it into lib/ for cgo to link against.
lib:
	cd $(LXP_SCAN_DIR) && cargo build --release
	cp $(LXP_SCAN_DIR)/target/release/$(DYLIB) lib/

# Regenerate the gRPC Go code from proto/ (needs protoc + the Go plugins on PATH).
proto:
	PATH="$$PATH:$(GOBIN)" protoc --proto_path=proto \
	  --go_out=. --go_opt=module=lxp-scan-svc \
	  --go-grpc_out=. --go-grpc_opt=module=lxp-scan-svc \
	  proto/lxpscan.proto

build:
	CGO_ENABLED=1 go build -o lxp-scan-svc .

run: build
	./lxp-scan-svc

clean:
	rm -f lxp-scan-svc lib/liblxp_scan.*
