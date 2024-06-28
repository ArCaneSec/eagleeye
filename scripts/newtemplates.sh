#!/bin/bash

CONFIG_FILE="/home/arcane/tools/config.sh"
source $CONFIG_FILE

CURRENT_PATH=$(pwd)

if [[ ! -d "$REPO_PATH" ]]; then
    echo "The path $REPO_PATH does not exist or is not a directory."
    exit 1
fi

# Clear old output files to avoid accumulating data from previous runs
> "$CURRENT_PATH"/old-files.txt
> "$CURRENT_PATH"/new-files.txt
> "$CURRENT_PATH"/changes.txt
> "$CURRENT_PATH"/all-new-templates.txt

cd $REPO_PATH

if ! git cat-file -e $LAST_COMMIT &>/dev/null; then
    echo "invalid commit: $LAST_COMMIT"
    exit 1
fi


# Listing all templates in repo
find . | grep -E '.*\.yaml$' | sort -u > "$CURRENT_PATH"/old-files.txt
git pull &>/dev/null
find . | grep -E '.*\.yaml$' | sort -u > "$CURRENT_PATH"/new-files.txt


# Get the recent commits excluding those with ':robot:' or 'Syncing Templates' since they are bot generated and junks
recentCommits=$(git log --oneline -100 | grep -v ':robot:' | grep -v 'Syncing Templates' | awk '{print $1}')

# Split the recent commits into an array
IFS=$'\n' read -d '' -r -a listOfCommits <<< "$recentCommits"


# Iterate over the array and add commit file's changes into changes.txt
for commit in "${listOfCommits[@]}";
do
    if [[ "$commit" == "$LAST_COMMIT" ]]; then
        break
    fi
    git diff-tree --no-commit-id --name-only -r $commit | grep -E '.*\.yaml$' >> "$CURRENT_PATH"/changes.txt
done

# Finding newly created templates
comm -23 "$CURRENT_PATH"/new-files.txt "$CURRENT_PATH"/old-files.txt >> "$CURRENT_PATH"/changes.txt

# Sort and unique
sort -u "$CURRENT_PATH"/changes.txt >> "$CURRENT_PATH"/all-new-templates.txt

# Removing junks
rm -rf "$CURRENT_PATH"/new-files.txt "$CURRENT_PATH"/old-files.txt "$CURRENT_PATH"/changes.txt

cat $CURRENT_PATH"/all-new-templates.txt"

# Updating config file
RECENT_COMMIT="${listOfCommits[0]}"
echo -e "#!/bin/bash\nREPO_PATH=$REPO_PATH\nLAST_COMMIT=$RECENT_COMMIT" > $CONFIG_FILE