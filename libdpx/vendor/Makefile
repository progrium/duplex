DIRECTORIES = $(sort $(dir $(wildcard */)))

all: setup $(DIRECTORIES)

setup:
	mkdir -pv .libs

$(DIRECTORIES): always
	$(foreach dir,$(DIRECTORIES),$(MAKE) -C $(dir); mv -v $(dir)/*.a .libs;)

always:
	@true

clean:
	$(foreach dir,$(DIRECTORIES),$(MAKE) -C $(dir) clean;)
	rm -rf .libs

.PHONY: all clean setup
