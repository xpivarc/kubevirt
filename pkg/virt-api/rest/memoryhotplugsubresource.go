package rest

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	restful "github.com/emicklei/go-restful"
	"k8s.io/apimachinery/pkg/api/errors"
	k8smetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/yaml"
	v1 "kubevirt.io/api/core/v1"
)

func (app *SubresourceAPIApp) MemoryHotplugVMIRequestHandler(request *restful.Request, response *restful.Response) {
	// TODO feature gate

	body := request.Request.Body
	if body == nil {
		writeError(errors.NewBadRequest("Body needs to be specified"), response)
		return
	}

	defer body.Close()

	// TODO validate that we can determine what resources should be bumped
	opt := v1.MemoryHotplugOption{}
	err := yaml.NewYAMLOrJSONDecoder(body, 1024).Decode(opt)
	if err != nil {
		writeError(errors.NewBadRequest(fmt.Sprintf("Invalid body, request cannot be decoded, error %s", err)), response)
		return
	}

	// TODO validate memory is bigger than before

	name := request.PathParameter("name")
	namespace := request.PathParameter("namespace")

	vmi, statusErr := app.FetchVirtualMachineInstance(namespace, name)
	if statusErr != nil {
		writeError(statusErr, response)
		return
	}

	oldValue, err := json.Marshal(vmi.Spec.Domain.Memory.Guest)
	if err != nil {
		writeError(errors.NewInternalError(err), response)
		return
	}

	vmi.Spec.Domain.Memory.Guest = &opt.Guest
	newValue, err := json.Marshal(vmi.Spec.Domain.Memory.Guest)
	if err != nil {
		writeError(errors.NewInternalError(err), response)
		return
	}

	test := fmt.Sprintf(`{ "op": "test", "path": "/spec/domain/memory/guest", "value": %s}`, string(oldValue))
	update := fmt.Sprintf(`{ "op": "%s", "path": "/spec/domain/memory/guest", "value": %s}`, "replace", string(newValue))
	patch := fmt.Sprintf("[%s, %s]", test, update)

	_, err = app.virtCli.VirtualMachineInstance(namespace).Patch(context.TODO(), name, types.JSONPatchType, []byte(patch), &k8smetav1.PatchOptions{})
	if err != nil {
		writeError(errors.NewInternalError(fmt.Errorf("unable to patch vm status during memory hotplug: %s", err)), response)
	}

	response.WriteHeader(http.StatusAccepted)
}
