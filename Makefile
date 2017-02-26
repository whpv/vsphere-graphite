GOVERSION=$(shell go version)
GOOS=$(word 1,$(subst /, ,$(lastword $(GOVERSION))))
GOARCH=$(word 2,$(subst /, ,$(lastword $(GOVERSION))))
RELEASE_DIR=releases
SRC_FILES=$(wildcard *.go)
BUILD_FLAGS=-ldflags '-s' -a 

deps:
	go get github.com/takama/daemon
	go get golang.org/x/net/context
	go get github.com/vmware/govmomi
	go get github.com/marpaia/graphite-golang
	go get github.com/influxdata/influxdb/client/v2

build-windows-amd64:
	@$(MAKE) build GOOS=windows GOARCH=amd64 SUFFIX=.exe

dist-windows-amd64:
	@$(MAKE) dist GOOS=windows GOARCH=amd64 SUFFIX=.exe

build-linux-amd64:
	@$(MAKE) build GOOS=linux GOARCH=amd64

dist-linux-amd64:
	@$(MAKE) dist GOOS=linux GOARCH=amd64

build-darwin-amd64:
	@$(MAKE) build GOOS=darwin GOARCH=amd64

dist-darwin-amd64:
	@$(MAKE) dist GOOS=darwin GOARCH=amd64
    
build-linux-arm:
	@$(MAKE) build GOOS=linux GOARCH=arm GOARM=5

dist-linux-arm:
	@$(MAKE) dist GOOS=linux GOARCH=arm GOARM=5

$(RELEASE_DIR)/$(GOOS)/$(GOARCH)/vsphere-graphite$(SUFFIX): $(SRC_FILES)
	go build $(BUILD_FLAGS) -o $(RELEASE_DIR)/$(GOOS)/$(GOARCH)/vsphere-graphite$(SUFFIX) .

$(RELEASE_DIR)/vsphere-graphite_$(GOOS)_$(GOARCH).tgz: $(RELEASE_DIR)/$(GOOS)/$(GOARCH)/vsphere-graphite$(SUFFIX)
	cd $(RELEASE_DIR)/$(GOOS)/$(GOARCH); tar czf ../../vsphere-graphite_$(GOOS)_$(GOARCH).tgz ./vsphere-graphite$(SUFFIX)

dist: $(RELEASE_DIR)/vsphere-graphite_$(GOOS)_$(GOARCH).tgz

build: $(RELEASE_DIR)/$(GOOS)/$(GOARCH)/vsphere-graphite$(SUFFIX)

clean:
	rm -rf $(RELEASE_DIR)
	
all:
	@$(MAKE) dist-windows-amd64 
	@$(MAKE) dist-linux-amd64
	@$(MAKE) dist-darwin-amd64
	@$(MAKE) dist-linux-arm
