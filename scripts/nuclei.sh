#!/bin/bash

nuclei -silent -nc -etags dns,ssl,technologies,tech -l $1 -duc -eid ./nuclei-junks.txt -c 5