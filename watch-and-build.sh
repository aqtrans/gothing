#!/bin/sh
CompileDaemon -exclude-dir=data -exclude-dir=.git -exclude-dir=vendor -include="*.tmpl" -command="./thing -d -l"
