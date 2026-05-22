set -x
cd /workspace

# FIXME: This script previously failed on retries because `git clone <url> .`
# requires the current directory to be COMPLETELY empty. The old logic
# preserved .tmp via `find ... -not -name '.tmp'`, leaving a non-empty
# directory and causing:
#   "fatal: destination path '.' already exists and is not an empty directory."
#
# The setup Job also used regular Containers (not InitContainers), so the
# parser container could fail independently and trigger a retry pod on the
# same PVC. The leftover .tmp from the first run then blocked all subsequent
# git-clone containers.
#
# Fixed by temporarily moving .tmp outside the workspace, cloning into an
# empty directory, then restoring .tmp afterwards.

# Check if .git exists to determine if a clone is needed
if [ ! -d ".git" ]
then
    # Temporarily stash .tmp outside the workspace so the directory is empty for cloning
    if [ -d ".tmp" ]
    then
        mv .tmp /tmp/workspace-tmp-backup
    fi

    # Remove everything else (including hidden files)
    find . -maxdepth 1 -mindepth 1 -exec rm -rf {} +
    ls -lah . # Verify cleanup

    if [[ $REPO_URL == "https://"* ]]; then
        git clone $REPO_URL . && git checkout $GIT_HASH_OR_BRANCH && echo "CLONED"
    else
        ssh-keyscan $REPO_DOMAIN >> /tmp/known_hosts
        git -c core.sshCommand='ssh -o UserKnownHostsFile=/tmp/known_hosts' clone $REPO_URL . && git checkout $GIT_HASH_OR_BRANCH && echo "CLONED"
    fi

    # Restore .tmp if it was stashed
    if [ -d "/tmp/workspace-tmp-backup" ]
    then
        mv /tmp/workspace-tmp-backup .tmp
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
