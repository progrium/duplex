.PHONY: test build

build:
	tsc; cp ./src/duplex.js ./dist/duplex.js;

test: build
	node_modules/.bin/jasmine-node --verbose test

demo: build
	cd demo && node server.js

dev:
	npm install
