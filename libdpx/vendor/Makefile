DIRECTORIES = $(sort $(dir $(wildcard */)))
OUR_DIR := $(shell dirname $(realpath $(lastword $(MAKEFILE_LIST))))

all: setup $(DIRECTORIES)

setup:
	mkdir -pv .libs

$(DIRECTORIES): always
	$(foreach dir,$(DIRECTORIES),cd $(dir); sh setup.sh; sh setup.sh install $(OUR_DIR)/.libs; cd ..;)

always:
	@true

clean:
	$(foreach dir,$(DIRECTORIES),cd $(dir); sh setup.sh clean; cd ..;)
	rm -rf .libs

.PHONY: all clean setup
