#!/usr/bin/env bash

set -e

##
# Print Usage
##
function usage() {
  echo "Usage:"
  echo "  ${BASH_SOURCE[0]} [OPTIONS]"
  echo ""

  echo "Options:"

  echo "  -b --branch "
  echo -e "\tThe protobuf tag."

  echo "  -h, --help"
  echo -e "\tDisplay this help."

  echo ""
  exit $1;
}

##
# Parse arguments
##
while [[ $# > 0 ]]
do
  case "$1" in
    -b|--branch) BRANCH=$2; shift ;;
    --gitlab-token) GITLAB_TOKEN=$2 ; shift ;;
    --help) usage ;;
  esac
  shift
done

[ -z "$BRANCH" ] && echo "BRANCH is required" && exit 1
[ -z "$GITLAB_TOKEN" ] && echo "GITLAB_TOKEN is required" && exit 1

rm -rf ./protobuf

git clone --depth 1 --branch "$BRANCH" "https://oauth2:$GITLAB_TOKEN@gitlab.infra.online.net/protobuf/protobuf.git"

buf generate

rm -rf ./protobuf

git add .
git commit -m "feat: update protobuf merge request"
git push origin "$BRANCH" -f

# Look which is the default branch
HOST="https://gitlab.infra.online.net/api/v4/projects"
TARGET_BRANCH="main"

# The description of our new MR, we want to remove the branch after the MR has
# been closed
BODY="{
    \"id\": ${CI_PROJECT_ID},
    \"source_branch\": \"${BRANCH}\",
    \"target_branch\": \"${TARGET_BRANCH}\",
    \"remove_source_branch\": true,
    \"title\": \"${BRANCH}\"
}";
echo "JSON PAYLOAD"
echo "$BODY"

# Require a list of all the merge request and take a look if there is already
# one with the same source branch
LIST_MR=$(curl "${HOST}/${CI_PROJECT_ID}/merge_requests?state=opened" --header "PRIVATE-TOKEN:${GITLAB_TOKEN}");
COUNT_BRANCHES=$(echo "${LIST_MR}" | grep -o "\"source_branch\":\"${CI_COMMIT_REF_NAME}\"" | wc -l);

# No MR found, let's create a new one
if [ "${COUNT_BRANCHES}" -eq "0" ]; then
    curl -X POST "${HOST}/${CI_PROJECT_ID}/merge_requests" \
        --header "PRIVATE-TOKEN:${GITLAB_TOKEN}" \
        --header "Content-Type: application/json" \
        --data "${BODY}";

    echo "Opened a new merge request: ${CI_COMMIT_REF_NAME}";
    exit;
fi

echo "No new merge request opened";
