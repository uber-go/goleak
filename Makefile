PACKAGES := $(shell glide nv)
# Many Go tools take file globs or directories as arguments instead of packages.
PACKAGE_FILES ?= *.go $(shell find internal -type f -iname '*.go')

.PHONY: build
build:
	go build -i $(PACKAGES)


.PHONY: install_deps
install_deps:
	glide --version || go get github.com/Masterminds/glide
	glide install


.PHONY: test
test:
	go test -v -race $(PACKAGES)

.PHONY: install_lint
install_lint:
	go get github.com/golang/lint/golint

.PHONY: lint
lint:
	@rm -rf lint.log
	@echo "Checking formatting..."
	@gofmt -d -s $(PACKAGE_FILES) 2>&1 | tee lint.log
	@echo "Checking vet..."
	@$(foreach dir,$(PACKAGE_FILES),go tool vet $(dir) 2>&1 | tee -a lint.log;)
	@echo "Checking lint..."
	@$(foreach dir,$(PACKAGES),golint $(dir) 2>&1 | tee -a lint.log;)
	@echo "Checking for unresolved FIXMEs..."
	@git grep -i fixme | grep -v -e '^vendor/' -e '^Makefile' | tee -a lint.log
	@[ ! -s lint.log ]

