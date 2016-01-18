#!/bin/sh
GO15VENDOREXPERIMENT=1 go run main.go handlers.go $@
