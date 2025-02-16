#!/bin/bash

# Find all files with merge conflict markers
files=$(grep -rlE '<<<<<<< |=======|>>>>>>>' --exclude='fix_merge_conflicts.sh')

if [ -z "$files" ]; then
    echo "No merge conflict markers found."
    exit 0
fi

echo "Fixing merge conflict markers in the following files:"
echo "$files"

# Loop through each file and remove the conflict markers
for file in $files; do
    echo "Cleaning $file..."
    sed -i '/<<<<<<< /d' "$file"
    sed -i '/=======/d' "$file"
    sed -i '/>>>>>>> /d' "$file"
done

echo "Merge conflict markers removed!"
