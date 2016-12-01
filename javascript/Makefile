.PHONY: test build

build:
	node_modules/.bin/coffee -o dist/ -c src/

test: build
	node_modules/.bin/jasmine-node --verbose --coffee test

demo: build
	cd demo && node server.js

dev:
	npm install
