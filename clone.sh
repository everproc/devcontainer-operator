set -x
cd /workspace

# Check if .git exists to determine if a clone is needed
if [ ! -d ".git" ]
then
    # If .git doesn't exist, ensure the directory is clean for cloning,
    # but explicitly keep the .tmp directory if it exists.
    # We remove all files and directories EXCEPT for .tmp and other dotfiles that should persist
    find . -maxdepth 1 -mindepth 1 -not -name '.tmp' -exec rm -rf {} +
    ls -lah . # Verify cleanup

    if [[ $REPO_URL == "https://"* ]]; then
        git clone $REPO_URL . && git checkout $GIT_HASH_OR_BRANCH && echo "CLONED"
    else
        ssh-keyscan $REPO_DOMAIN >> /tmp/known_hosts
        git -c core.sshCommand='ssh -o UserKnownHostsFile=/tmp/known_hosts' clone $REPO_URL . && git checkout $GIT_HASH_OR_BRANCH && echo "CLONED"
    fi

    if [ ! -d "/workspace/.tmp/git_status" ]
    then
        mkdir -p /workspace/.tmp/git_status
    fi
    git rev-parse HEAD > /workspace/.tmp/git_status/clone_done && ls . && echo "DONE"
else
    ls -lah
    echo "Already cloned"
    if [ ! -d "/workspace/.tmp/git_status" ]
    then
        mkdir -p /workspace/.tmp/git_status
    fi
    git rev-parse HEAD > /workspace/.tmp/git_status/clone_done
    exit 0
fi
