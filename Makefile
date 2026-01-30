KIWI_VERSION := v0.21.0

.PHONY: all
all:

.PHONY: clean
clean:

.PHONY: build
build:

.PHONY: run
run:

.PHONY: install-kiwi
install-kiwi:
	./scripts/install_kiwi.bash $(KIWI_VERSION)

.PHONY: test
test:
	./scripts/run_tests.bash
