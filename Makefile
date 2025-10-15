# Makefile for GIMini GIMP Plugin

# The final binary name for the plugin
PLUGIN_NAME = GIMini

# Go source file
SRC = main.go

# Default target
all: build

build:
	@echo "Building GIMini plugin..."
	go build -o $(PLUGIN_NAME) -buildmode=c-shared $(SRC)
	@echo "Build complete: $(PLUGIN_NAME)"

clean:
	@echo "Cleaning up build artifacts..."
	rm -f $(PLUGIN_NAME) *.h gimini-debug.log
