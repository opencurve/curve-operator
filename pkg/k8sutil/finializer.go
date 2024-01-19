package k8sutil

import (
	"context"
	"fmt"
	"strings"

	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const CustomResourceGroup = "curve.opencurve.io"

// AddFinalizerIfNotPresent adds a finalizer an object to avoid instant deletion
// of the object without finalizing it.
func AddFinalizerIfNotPresent(ctx context.Context, cli client.Client, obj runtime.Object) error {
	objectFinalizer := buildFinalizerName(obj.GetObjectKind().GroupVersionKind().Kind)
	accessor, err := meta.Accessor(obj)
	if err != nil {
		return errors.Wrap(err, "failed to get meta information of object")
	}

	if !contains(accessor.GetFinalizers(), objectFinalizer) {
		logger.Infof("adding finalizer %q on %q", objectFinalizer, accessor.GetName())
		accessor.SetFinalizers(append(accessor.GetFinalizers(), objectFinalizer))
		// Update CR with finalizer
		err = cli.Update(ctx, obj)
		if err != nil {
			return errors.Wrapf(err, "failed to add finalizer %q on %q", objectFinalizer, accessor.GetName())
		}
	}

	return nil
}

// RemoveFinalizer removes a finalizer from an object
func RemoveFinalizer(ctx context.Context, client client.Client, namespacedName types.NamespacedName, obj runtime.Object) error {
	finalizerName := buildFinalizerName(obj.GetObjectKind().GroupVersionKind().Kind)
	return RemoveFinalizerWithName(ctx, client, namespacedName, obj, finalizerName)
}

// RemoveFinalizerWithName removes finalizer passed as an argument from an object
func RemoveFinalizerWithName(ctx context.Context, client client.Client, namespacedName types.NamespacedName, obj runtime.Object, finalizerName string) error {
	err := client.Get(ctx, namespacedName, obj)
	if err != nil {
		return errors.Wrap(err, "failed to get the latest version of the object")
	}
	accessor, err := meta.Accessor(obj)
	if err != nil {
		return errors.Wrap(err, "failed to get meta information of object")
	}

	if contains(accessor.GetFinalizers(), finalizerName) {
		logger.Infof("removing finalizer %q on %q", finalizerName, accessor.GetName())
		accessor.SetFinalizers(remove(accessor.GetFinalizers(), finalizerName))
		if err := client.Update(ctx, obj); err != nil {
			return errors.Wrapf(err, "failed to remove finalizer %q on %q", finalizerName, accessor.GetName())
		}
	}

	return nil
}

// buildFinalizerName returns the finalizer name
func buildFinalizerName(kind string) string {
	return fmt.Sprintf("%s.%s", strings.ToLower(kind), CustomResourceGroup)
}

// contains checks if an item exists in a given list.
func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}

	return false
}

// remove removes any element from a list
func remove(list []string, s string) []string {
	for i, v := range list {
		if v == s {
			list = append(list[:i], list[i+1:]...)
		}
	}

	return list
}
