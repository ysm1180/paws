.PHONY: build run clean install uninstall

BIN_NAME := paws
INSTALL_PATH := /usr/local/bin

build:
	go build -o $(BIN_NAME) .

run:
	go run .

clean:
	rm -f $(BIN_NAME)

install: build
	sudo cp $(BIN_NAME) $(INSTALL_PATH)/

uninstall:
	sudo rm -f $(INSTALL_PATH)/$(BIN_NAME)
