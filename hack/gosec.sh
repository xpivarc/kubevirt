source hack/common.sh

export ARTIFACTS=$KUBEVIRT_DIR/${ARTIFACTS:-_out/artifacts}
mkdir -p $ARTIFACTS

echo "Run go sec in pkg"
cd $KUBEVIRT_DIR/pkg
set -x
gosec -exclude-dir=testutils -fmt=junit-xml -out=junit-gosec.xml ./... 