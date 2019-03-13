#!/bin/sh
go build -o gothing
debuild -us -uc -b
