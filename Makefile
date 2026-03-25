.PHONY: tui build arc prepare-embedded-helpers

CLIPD_MANIFEST := clipd/Cargo.toml
CLIPD_RELEASE := clipd/target/release/arc-clipd
CLIPD_EMBED_PATH := src/embedded/arc-clipd

prepare-embedded-helpers:
	mkdir -p src/embedded
	cargo build --manifest-path $(CLIPD_MANIFEST) --release --offline || cargo build --manifest-path $(CLIPD_MANIFEST) --release
	cp $(CLIPD_RELEASE) $(CLIPD_EMBED_PATH)

# Default target
tui: prepare-embedded-helpers
	cd src && go run -tags arc_clipd_embed .

build: prepare-embedded-helpers
	mkdir -p bin
	cd src && go build -tags arc_clipd_embed -o ../bin/arc .

arc: build
	./bin/arc $(ARGS)
