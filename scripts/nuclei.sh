#!/bin/bash

nuclei -silent -nc -etags dns,ssl,technologies,tech -l $2 -duc -eid ./nuclei-junks.txt