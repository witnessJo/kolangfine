KIWI_VERSION := v0.21.0
.PHONY: all
all: install-model build

.PHONY: clean
clean:

.PHONY: build
build:
	go build -o kolangfine ./main.go

.PHONY: run
run:
	go run ./main.go

.PHONY: install-kiwi
install-kiwi:
	./scripts/install_kiwi.bash $(KIWI_VERSION)

.PHONY: install-model
install-model:
	curl -L https://github.com/bab2min/Kiwi/releases/download/$(KIWI_VERSION)/kiwi_model_$(KIWI_VERSION)_base.tgz --output model.tgz
	tar --no-same-owner -xzvf model.tgz
	rm -f model.tgz

.PHONY: test
test:
	./scripts/run_tests.bash
