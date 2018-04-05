#!/bin/sh
echo $@ >> /run/hookscheck
read line
echo $line >> /run/hookscheck
