set -x
mkdir -p /workspace


cd /workspace
ls -lah .
if [ ! -d ".git" ]
then
    git clone $REPO_URL . && git checkout $GIT_HASH_OR_BRANCH echo "CLONED" 

    if [ ! -d "/workspace/.tmp/git_status" ]
    then
        mkdir -p /workspace/.tmp/git_status
    fi
    touch /workspace/.tmp/git_status/clone_done && ls . && echo "DONE"
else
    ls -lah
    echo "Already cloned"
    if [ ! -d "/workspace/.tmp/git_status" ]
    then
        mkdir -p /workspace/.tmp/git_status
    fi
    touch /workspace/.tmp/git_status/clone_done
    exit 0
fi
