#!/usr/bin/env bash

set -e

# Configure ssh keys and known_hosts
eval $(ssh-agent -s)
vault kv get -field ssh_private_key developer-tools_kv/gitlab | ssh-add - > /dev/null
mkdir -p ~/.ssh/
ssh-keyscan -t rsa github.com >> ~/.ssh/known_hosts

# Configure git for scaleway-bot github account
git config --global user.name 'scaleway-bot'
git config --global user.email 'github@scaleway.com'

SDK_PRODUCTS="
account/v2alpha1
applesilicon/v1alpha1
baremetal/v1
baremetal/v1alpha1
container/v1beta1
domain/v2alpha2
domain/v2beta1
instance/v1
iot/v1
k8s/v1
k8s/v1beta3
k8s/v1beta4
lb/v1
marketplace/v1
rdb/v1
registry/v1
vpc/v1
vpcgw/v1
vpcgw/v1beta1"

set -x

git remote remove gitlab || true
git remote add gitlab https://oauth2:"${GITLAB_TOKEN}"@gitlab.infra.online.net/protobuf/scaleway-sdk-go.git

git remote remove upstream || true
git remote add upstream git@github.com:scaleway/scaleway-sdk-go.git

git remote remove scaleway-bot || true
git remote add scaleway-bot git@github.com:scaleway-bot/scaleway-sdk-go.git

git fetch --all --prune

# Rebase fork from upstream
git checkout upstream/master

# check the tag variable exists
[ -z "${TAG}" ] && echo "No TAG variable exists" && exit 1
git branch -D "${TAG}" || true
git checkout -b "${TAG}"

# Generate SDK
rm -rf ./protobuf
git clone -v --depth 1 --branch $TAG https://oauth2:"${GITLAB_TOKEN}"@gitlab.infra.online.net/protobuf/protobuf.git
# We extract the buf configuration files on the internal protobuf/scaleway-sdk-go to generate the sdk
git checkout gitlab/master -- buf.gen.yaml buf.lock buf.yaml buf.work.yaml
# We don't want to add the buf config files to the commit sent to GitHub
git reset buf.gen.yaml buf.lock buf.work.yaml buf.yaml
buf generate

# get commit message (skip ci tag message)
commit=$(git log --pretty='format:%Creset%s' --no-merges --skip=1 -1)
commit_title="feat: update generated apis"

# transform "baremetal: add new field" in "feat(baremetal): add new field" if the commit starts with "baremetal:"
for product in ${SDK_PRODUCTS}; do
    # remove version to get only the product
    product_name=${product%"/"*}
    if [[ "$commit" == "${product_name}:"* ]]; then
        commit_title="feat(${product_name}):${commit#"${product_name}:"}"
        break
    fi
done

# Commit and push generated files
for product in ${SDK_PRODUCTS}; do
  git add "api/${product}"
done

git add api/test/v1

git status api

# Exit if git stage is empty
[ -z "$(git diff --cached  --exit-code api)" ] && echo "git stage is empty. nothing to push" && exit 0

# If not commit and push to scaleway-bot repository (git@github.com:scaleway-bot/scaleway-sdk-go.git)
git commit -m "${commit_title}"
git push --force scaleway-bot "${TAG}"

# Open pull request on Github
PULL_URL="https://api.github.com/repos/scaleway/scaleway-sdk-go/pulls"
SCALEWAY_BOT_GITHUB_TOKEN=$(vault kv get -field access_token developer-tools_kv/github)
HEAD="scaleway-bot:${TAG}"
BASE="master"
DATA='{"title":"'${commit_title}'", "head":"'$HEAD'", "base":"'$BASE'", "maintainer_can_modify":true}'
curl --fail -i $PULL_URL -X POST -H "Authorization:token $SCALEWAY_BOT_GITHUB_TOKEN" -H "Content-Type:application/json" --data "$DATA"
