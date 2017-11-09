SOURCEDIR=.
SOURCES := $(shell find $(SOURCEDIR) -name '*.go')

BINARY=build/main

.DEFAULT_GOAL: $(BINARY)

$(BINARY): $(SOURCES)
	CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -v -o ${BINARY} .