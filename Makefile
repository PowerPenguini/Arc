.PHONY: tui build

# Default target
tui:
	go run ./src

build:
	mkdir -p bin
	go build -o bin/arc ./src
