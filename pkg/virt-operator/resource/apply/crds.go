package apply

import (
	"context"
	"encoding/json"
	"fmt"

	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"kubevirt.io/client-go/log"
)

func getSubresourcesForVersion(crd *extv1.CustomResourceDefinition, version string) *extv1.CustomResourceSubresources {
	for _, v := range crd.Spec.Versions {
		if version == v.Name {
			return v.Subresources
		}
	}
	return nil
}

func needsSubresourceStatusEnable(crd, cachedCrd *extv1.CustomResourceDefinition) bool {
	for _, version := range crd.Spec.Versions {
		if version.Subresources != nil && version.Subresources.Status != nil {
			subresource := getSubresourcesForVersion(cachedCrd, version.Name)
			if subresource == nil || subresource.Status == nil {
				return true
			}
		}
	}
	return false
}

func needsSubresourceStatusDisable(crdTargetVersion *extv1.CustomResourceDefinitionVersion, cachedCrd *extv1.CustomResourceDefinition) bool {
	// subresource support needs to be introduced carefully after the control plane roll-over
	// to avoid creating zombie entities which don't get processed due to ignored status updates
	cachedSubresource := getSubresourcesForVersion(cachedCrd, crdTargetVersion.Name)
	return (cachedSubresource == nil || cachedSubresource.Status == nil) &&
		(crdTargetVersion.Subresources != nil && crdTargetVersion.Subresources.Status != nil)
}

func patchCRD(client clientset.Interface, crd *extv1.CustomResourceDefinition, ops []string) error {
	newSpec, err := json.Marshal(crd.Spec)
	if err != nil {
		return err
	}

	if ops == nil {
		ops = make([]string, 1)
	}
	ops = append(ops, fmt.Sprintf(`{ "op": "replace", "path": "/spec", "value": %s }`, string(newSpec)))

	_, err = client.ApiextensionsV1().CustomResourceDefinitions().Patch(context.Background(), crd.Name, types.JSONPatchType, generatePatchBytes(ops), metav1.PatchOptions{})
	if err != nil {
		return fmt.Errorf("unable to patch crd %+v: %v", crd, err)
	}

	log.Log.V(2).Infof("crd %v updated", crd.GetName())
	return nil
}

func (r *Reconciler) createOrUpdateCrds() error {
	for _, crd := range r.targetStrategy.CRDs() {
		err := r.createOrUpdateCrd(crd)
		if err != nil {
			return err
		}
	}

	return nil
}

func (r *Reconciler) createOrUpdateCrd(crd *extv1.CustomResourceDefinition) error {
	client := r.clientset.ExtensionsClient()
	version, imageRegistry, id := getTargetVersionRegistryID(r.kv)
	var cachedCrd *extv1.CustomResourceDefinition

	crd = crd.DeepCopy()
	injectOperatorMetadata(r.kv, &crd.ObjectMeta, version, imageRegistry, id, true)
	obj, exists, _ := r.stores.CrdCache.Get(crd)
	if !exists {
		// Create non existent
		r.expectations.Crd.RaiseExpectations(r.kvKey, 1, 0)
		_, err := client.ApiextensionsV1().CustomResourceDefinitions().Create(context.Background(), crd, metav1.CreateOptions{})
		if err != nil {
			r.expectations.Crd.LowerExpectations(r.kvKey, 1, 0)
			return fmt.Errorf("unable to create crd %+v: %v", crd, err)
		}
		log.Log.V(2).Infof("crd %v created", crd.GetName())
		return nil
	} else {
		cachedCrd = obj.(*extv1.CustomResourceDefinition)
	}

	if !objectMatchesVersion(&cachedCrd.ObjectMeta, version, imageRegistry, id, r.kv.GetGeneration()) {
		// Patch if old version
		for i := range crd.Spec.Versions {
			if needsSubresourceStatusDisable(&crd.Spec.Versions[i], cachedCrd) {
				crd.Spec.Versions[i].Subresources.Status = nil
			}
		}
		// Add Labels and Annotations Patches
		var ops []string
		labelAnnotationPatch, err := createLabelsAndAnnotationsPatch(&crd.ObjectMeta)
		if err != nil {
			return err
		}
		ops = append(ops, labelAnnotationPatch...)
		if err := patchCRD(client, crd, ops); err != nil {
			return err
		}
		log.Log.V(2).Infof("crd %v updated", crd.GetName())
	} else {
		log.Log.V(2).Infof("crd %v is up-to-date", crd.GetName())
	}
	return nil
}

func (r *Reconciler) rolloutNonCompatibleCRDChanges() error {
	for _, crd := range r.targetStrategy.CRDs() {
		err := r.rolloutNonCompatibleCRDChange(crd)
		if err != nil {
			return err
		}
	}

	return nil
}

func (r *Reconciler) rolloutNonCompatibleCRDChange(crd *extv1.CustomResourceDefinition) error {
	client := r.clientset.ExtensionsClient()
	version, imageRegistry, id := getTargetVersionRegistryID(r.kv)
	var cachedCrd *extv1.CustomResourceDefinition

	crd = crd.DeepCopy()
	obj, exists, err := r.stores.CrdCache.Get(crd)
	if !exists {
		return err
	}

	cachedCrd = obj.(*extv1.CustomResourceDefinition)
	injectOperatorMetadata(r.kv, &crd.ObjectMeta, version, imageRegistry, id, true)
	if objectMatchesVersion(&cachedCrd.ObjectMeta, version, imageRegistry, id, r.kv.GetGeneration()) {
		// Patch if in the deployed version the subresource is not enabled
		if !needsSubresourceStatusEnable(crd, cachedCrd) {
			return nil
		}
		// enable the status subresources now, in case that they were disabled before
		if err := patchCRD(client, crd, nil); err != nil {
			return err
		}
	}
	log.Log.V(4).Infof("crd %v is up-to-date", crd.GetName())
	return nil
}
