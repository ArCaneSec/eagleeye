#!/bin/bash

nuclei -silent -duc -up &>/dev/null
if [ $? -ne 0 ]; then
    echo 'An error occured while updating nuclei.'
    exit 1
fi

CONFIG_FILE=$1 #"/home/arcane/tools/config.sh"
source $CONFIG_FILE

CURRENT_PATH=$(pwd)

if [[ ! -d "$REPO_PATH" ]]; then
    echo "The path $REPO_PATH does not exist or is not a directory."
    exit 1
fi

# Clear old output files to avoid accumulating data from previous runs
> "$OLD_FILES" #/old-files.txt
> "$NEW_FILES" #/new-files.txt
> "$CHANGES" #/changes.txt
> "$NEW_TEMPLATES" #/all-new-templates.txt

cd $REPO_PATH

if ! git cat-file -e $LAST_COMMIT &>/dev/null; then
    echo "invalid commit: $LAST_COMMIT"
    exit 1
fi


# Listing all templates in repo
find . | grep -E '.*\.yaml$' | sort -u | sed 's/^\.\///' | xargs -I {} echo "$REPO_PATH"{} > "$OLD_FILES"
git pull &>/dev/null
find . | grep -E '.*\.yaml$' | sort -u | sed 's/^\.\///' | xargs -I {} echo "$REPO_PATH"{} >  "$NEW_FILES"


# Get the recent commits excluding those with ':robot:' or 'Syncing Templates' since they are bot generated and junks
recentCommits=$(git log --oneline -10 | grep -v ':robot:' | grep -v 'Syncing Templates' | awk '{print $1}')

# Split the recent commits into an array
IFS=$'\n' read -d '' -r -a listOfCommits <<< "$recentCommits"


# Iterate over the array and add commit file's changes into changes.txt
for commit in "${listOfCommits[@]}";
do
    if [[ "$commit" == "$LAST_COMMIT" ]]; then
        break
    fi
    git diff-tree --no-commit-id --name-status -r $commit | grep -E '^[AM]\s.*\.yaml$' | awk '{print $2}' | xargs -I {} echo "$REPO_PATH"{} >> "$CHANGES"
done

# Finding newly created templates
comm -23 "$NEW_FILES" "$OLD_FILES" >> "$CHANGES"

# Sort and unique
sort -u "$CHANGES" >> "$NEW_TEMPLATES"

# Removing junks
rm -rf "$NEW_FILES" "$OLD_FILES" "$CHANGES"

# cat "$NEW_TEMPLATES"

# Updating config file
RECENT_COMMIT="${listOfCommits[0]}"
echo -e "\
#!/bin/bash\nREPO_PATH=$REPO_PATH\nLAST_COMMIT=$RECENT_COMMIT
OLD_FILES=$OLD_FILES\nNEW_FILES=$NEW_FILES\nCHANGES=$CHANGES
NEW_TEMPLATES=$NEW_TEMPLATES" > $CONFIG_FILE

cat "$NEW_TEMPLATES" | wc -l