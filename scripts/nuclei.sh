#!/bin/bash

# nuclei -silent -etags dns,ssl,technologies,tech -t $1 -l $2 -duc -eid ./nuclei-junks.txt
nuclei -silent -nc -etags dns,ssl,technologies,tech -l $2 -duc -eid ./nuclei-junks.txt