#!/usr/bin/env bash

source hack/common.sh
export ARTIFACTS=$KUBEVIRT_DIR/${ARTIFACTS:-_out/artifacts}
if ${GENERATE} == "true"; then
    
    mkdir -p $ARTIFACTS

    echo "Run go sec in pkg"
    cd $KUBEVIRT_DIR/pkg

    # -confidence=high -severity=high <- for filtering
    gosec -out=${ARTIFACTS}/junit-gosec.xml -no-fail -exclude-dir=testutils  -fmt=junit-xml  ./... 

    set -x
    cd ${KUBEVIRT_DIR}/tools/gosec
    go build
    ./gosec -junit="${ARTIFACTS}/junit-gosec.xml" 
    ./gosec -junit="${ARTIFACTS}/junit-gosec.xml"
    ./gosec -junit="${ARTIFACTS}/junit-gosec.xml"
else
    cd ${KUBEVIRT_DIR}/tools/gosec
    diff "${ARTIFACTS}/junit-gosec.xml" "junit-gosec.xml" 
fi