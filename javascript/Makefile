.PHONY: test build

build:
	coffee -o dist/ -c src/

test: build
	jasmine-node --verbose --coffee test

demo: build
	cd demo && node server.js
