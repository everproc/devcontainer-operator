set -x
cd /workspace
rm -rf *
ls -lah .
if [ ! -d ".git" ]
then
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
