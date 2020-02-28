.PHONY: build
build:
	echo "Building binary"
	rm core-ci || true
	go build -o core-ci .

run:
	echo "Cleaning old logs directory"
	rm -rf logs/* || true
	./core-ci
