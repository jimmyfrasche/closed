#!/bin/bash

for p in $(go list std)
do
	echo $p
	closed-explorer $p
done
