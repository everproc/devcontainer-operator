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
fi
if [ ! -d "/workspace/.tmp/git_status" ]
then
    mkdir -p /workspace/.tmp/git_status
fi
git rev-parse HEAD > /workspace/.tmp/git_status/clone_done
cat /workspace/.tmp/git_status/clone_done
ls -lah .
echo "DONE"
exit 0
