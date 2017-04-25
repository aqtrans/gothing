#!/bin/sh
cd scss/
npm install
bower --allow-root install
npm build
gulp sass
cd ../