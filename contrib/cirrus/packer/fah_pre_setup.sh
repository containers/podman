#!/bin/bash

# ... stuff ...

sudo setenforce 0

sudo atomic host upgrade --reboot
