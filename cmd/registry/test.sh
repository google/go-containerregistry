#!/bin/bash
set -ex

CONTAINER_OS=$(docker info -f '{{ .OSType }}')

# crane can run on a Windows system, but doesn't currently support pulling Windows
# containers, so this test can only run if Docker is in Linux container mode.
if [[ ${CONTAINER_OS} = "windows" ]]; then
    set +x
    echo [TEST SKIPPED] Windows containers are not yet supported by crane
    exit
fi

function cleanup {
    [[ -n $PID ]] && kill $PID
    [[ -n $CTR ]] && docker stop $CTR
    rm -f ubuntu.tar debiand.tar debianc.tar
    docker rmi -f \
        localhost:1338/debianc:latest \
        localhost:1338/debiand:latest \
        localhost:1338/ubuntuc:foo \
        localhost:1338/ubuntud:latest \
        || true
}
trap cleanup EXIT

case "$OSTYPE" in
    # On Windows, Docker runs in a VM, so a registry running on the Windows
    # host is not accessible via localhost for `docker pull|push`.
    win*|msys*|cygwin*)
        docker run -d --rm -p 1338:5000 --name test-reg registry:2
        CTR=test-reg
        ;;

    *)
        registry &
        PID=$!
        ;;
esac

go install ./cmd/registry
go install ./cmd/crane


crane pull debian:latest debianc.tar
crane push debianc.tar localhost:1338/debianc:latest
docker pull localhost:1338/debianc:latest
docker tag localhost:1338/debianc:latest localhost:1338/debiand:latest
docker push localhost:1338/debiand:latest
crane pull localhost:1338/debiand:latest debiand.tar

docker pull ubuntu:latest
docker tag ubuntu:latest localhost:1338/ubuntud:latest
docker push localhost:1338/ubuntud:latest
crane pull localhost:1338/ubuntud:latest ubuntu.tar
crane push ubuntu.tar localhost:1338/ubuntuc:foo
docker pull localhost:1338/ubuntuc:foo
