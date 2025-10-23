SUBDIRS=20250814-KeylimeMCP-Inception

.PHONY: all
all:
	@for dir in $(SUBDIRS); do \
		make -C $$dir; \
	done

.PHONY: clean
clean:
	@for dir in $(SUBDIRS); do \
		make -C $$dir clean; \
	done

.PHONY: test
test:
	@for dir in $(SUBDIRS); do \
		make -C $$dir test; \
	done
