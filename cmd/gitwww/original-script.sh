#!/bin/bash

# This script checks for new Git commits every minute. Any new commit triggers the Containerfile build and copies out the files inside the container into folder $WWW_DIR

ENGINE=${ENGINE:-docker} # Container manager: podman, buildah or docker
export DOCKER_BUILDKIT=${DOCKER_BUILDKIT:-1}

GIT_REPO=${GIT_REPO:-/path/to/repo}
ADDR=${ADDR:-https://example.com/path}
PORT=${PORT:-8080}
WWW_DIR=${WWW_DIR:-/var/opt/www}

set -e                   # -o errexit  stop the script when an error occurs
set -u                   # -o nounset  unset variable -> error
set -o pipefail          # exit code of a pipeline is the first with a non-zero status (or zero if all commands exit successfully)
shopt -s inherit_errexit # Also apply restrictions to $(command substitution)

log() { set +x; echo -e "\033[34m$(date +%H:%M)\033[m \033[32m" "$@" "\033[m"; }
err() { set +x; echo -e "\033[34m$(date +%H:%M)\033[m \033[31m" "$@" "\033[m"; }

OK="$(log Back to normal)"
KO="$(err Regression)"

status="$OK"
prev="$OK"

main() {
    cd "$GIT_REPO"
    log "Monitoring directories: $GIT_REPO"

    for (( ; ; )); do
        for d in "$@"
        do
        (
            cd "$d"
            if is_wwwdir_empty "$d" || is_there_a_new_commit
            then
                pull_build_deploy "$d" || err "Error during pull_build_deploy"
            fi
        )
        done

        log "Wait 1 minute for new commit"
        sleep 60 || true  # true in order to "pkill sleep" without stopping the script
    done
}

is_wwwdir_empty() {
    local dir="$WWW_DIR/${1:?Missing argument}"
    if ls -qA "$dir" 2>/dev/null | grep -q .; then
        return 1  # 1=Failure (false) => directory contains something
    else
        log "Missing or empty directory: $dir"
        return 0  # 0=Success (true)
    fi
}

is_there_a_new_commit() {
    (
        set -x
        git fetch
    )

    remote="$(git rev-parse "@{upstream}")"
    local="$( git rev-parse HEAD)"

    if [[ "$remote" != "$local" ]]; then
        commit_msg="$(git log -1 --pretty=format:%f)"
        log "New commit: $commit_msg"
        return 0  # 0=Success (true)
    else
        return 1  # 1=Failure (false)
    fi
}

pull_build_deploy() {
    if (
        set -x
        git pull --ff-only ||
        git reset --hard origin/main
    ); then
        : # Do nothing
    else
        err "KO git pull. Local changes? Files owned by root? SSH public key?"
    fi

    commit_msg="$(git log -1 --pretty=format:%f)"

    file=
    if [[ ! -f "Dockerfile" ]] ; then
        file="-f $(find -regex ".*\(Contain\|Dock\)erfile.*" -print -quit)"
    fi

    if (
        set  -x
        $ENGINE build --build-arg addr="$ADDR" --build-arg port="$PORT" $file -t "$1" "$1"
    ); then
        status="$OK"
        log "OK build commit: $commit_msg"
        deploy "$1" || log "No change in dist files"
    else
        status="$KO"
        email="$(git log -1 --pretty=format:%ae)"
        name="$(git log -1 --pretty=format:%an)"
        err "KO commit: $commit_msg - $name $email"
    fi

    if [[ $status != "$prev" ]]; then
        prev="$status"
        echo "$status"
        # send e-mail
    fi
}

deploy() { (
    local dir="$WWW_DIR/${1:?Missing argument}"
    set -x

    mkdir -pv "$dir"
    rm -rfv "$dir.new"

    # last "" is the required command
    container_id=$($ENGINE create "$1" "")
    $ENGINE cp "$container_id:/dist" "$dir.new"
    $ENGINE rm "$container_id"

    # Stop here if same files
    diff -rq "$dir" "$dir.new" && return 1

    # Switch folders, this should be fast
    rm -rf        "$dir.old"
    mv "$dir"     "$dir.old"
    mv "$dir.new" "$dir"
    # Do not remove "$dir.old" because other apps may still access some file-descriptors within this folder

    tree -hC  "$dir"
); }

main "$@"
