#!/bin/sh
cd ../
go build -o debbuild/gothing
cd debbuild/
debuild -us -uc -b
