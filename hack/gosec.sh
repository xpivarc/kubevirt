source hack/common.sh


echo "Run go sec in pkg"
cd $KUBEVIRT_DIR/pkg
gosec -exclude-dir=testutils -fmt=junit-xml -out=output.xml ./... 
