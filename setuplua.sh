#!/bin/bash
set -e
mkdir -p lua_vendor/ # can't be bothered to tell go to ignore vendor/
cd lua_vendor/
git clone https://github.com/t7ru/scribunto-luacats
echo "done!"
