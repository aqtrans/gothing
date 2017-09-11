#!/bin/sh

## Using CompileDaemon, only building main.go:
#CompileDaemon -exclude-dir=md -exclude-dir=md2 -exclude-dir=.git -exclude-dir=vendor -include="*.tmpl" -command="./wiki -d"

## Using reflex:
#go get github.com/cespare/reflex
#reflex -c reflex.conf

## Using Entr (http://entrproject.org/), as reflex wasn't working properly on OSX
#ls scss/*.scss | entr sass scss/grid.scss assets/css/wiki.css
#### Rebuilding SASS and Go in one
#ls -d main.go main_test.go templates/** scss/grid.scss | entr -r sh -c 'sass scss/grid.scss assets/css/wiki.css && go run main.go'

## Using Modd (https://github.com/cortesi/modd) now, which allows multiple commands, multiple matches
go get github.com/cortesi/modd/cmd/modd
modd 
