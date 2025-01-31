#!/usr/bin/make -f

PWD=$(shell pwd)

build:
	go build -o servmon-bin .

install:
	go install .