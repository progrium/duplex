DIRECTORIES = $(sort $(dir $(wildcard */)))
OUR_DIR := $(shell dirname $(realpath $(lastword $(MAKEFILE_LIST))))

all: .libs

.libs: $(DIRECTORIES)
	mkdir -pv .libs
	$(foreach dir,$(DIRECTORIES),cd $(dir); sh setup.sh; sh setup.sh install $(OUR_DIR)/.libs; cd ..;)

clean:
	$(foreach dir,$(DIRECTORIES),cd $(dir); sh setup.sh clean; cd ..;)
	rm -rf .libs
	rm -f .setup

.PHONY: all clean
