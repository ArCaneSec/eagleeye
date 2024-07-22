#!/bin/bash

nuclei -silent -nc -etags dns,ssl,technologies,tech -l $1 -duc -t $2 -c 5