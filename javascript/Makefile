.PHONY: test build

build:
	tsc; mv ./src/duplex.js ./dist/duplex.js;
	cd test && tsc

test:
	node_modules/.bin/jasmine-node --verbose test

demo: build
	cd demo && node server.js

dev:
	npm install
