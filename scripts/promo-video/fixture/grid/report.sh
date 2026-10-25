#!/bin/sh
# Collect the grid's final losses: how many runs reported, and the best three.
echo "report: $(ls results | wc -l) runs"
sort -n results/*.txt | head -3
