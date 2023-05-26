package controllers

import (
	"context"
	"fmt"
	"strings"

	"github.com/pkg/errors"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	curvev1 "github.com/opencurve/curve-operator/api/v1"
)

func removeFinalizer(cli client.Client, name types.NamespacedName, obj client.Object, finalizer string) error {
	err := cli.Get(context.Background(), name, obj)
	if err != nil {
		if kerrors.IsNotFound(err) {
			logger.Debugf("%s resource not found. Ignoring since object must be deleted.", name.Name)
			return nil
		}
		return errors.Wrapf(err, "failed to retrieve %q to remove finalizer", name.Name)
	}

	if finalizer == "" {
		err = RemoveFinalizer(context.Background(), cli, obj, name)
		if err != nil {
			return err
		}
	} else {
		err = RemoveFinalizerWithName(context.Background(), cli, obj, name, finalizer)
		if err != nil {
			return err
		}
	}
	return nil
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

// AddFinalizerIfNotPresent adds a finalizer an object to avoid instant deletion
// of the object without finalizing it.
func AddFinalizerIfNotPresent(ctx context.Context, cli client.Client, obj client.Object) error {
	objectFinalizer := buildFinalizerName(obj.GetObjectKind().GroupVersionKind().Kind)
	accessor, err := meta.Accessor(obj)
	if err != nil {
		return errors.Wrap(err, "failed to get meta information of object")
	}

	if !contains(accessor.GetFinalizers(), objectFinalizer) {
		logger.Infof("adding finalizer %q on %q", objectFinalizer, accessor.GetName())
		accessor.SetFinalizers(append(accessor.GetFinalizers(), objectFinalizer))
		// Update CR with finalizer
		if err := cli.Update(ctx, obj); err != nil {
			return errors.Wrapf(err, "failed to add finalizer %q on %q", objectFinalizer, accessor.GetName())
		}
	}

	return nil
}

// RemoveFinalizer removes a finalizer from an object
func RemoveFinalizer(ctx context.Context, cli client.Client, obj client.Object, namespacedName types.NamespacedName) error {
	finalizerName := buildFinalizerName(obj.GetObjectKind().GroupVersionKind().Kind)
	return RemoveFinalizerWithName(ctx, cli, obj, namespacedName, finalizerName)
}

// RemoveFinalizerWithName removes finalizer passed as an argument from an object
func RemoveFinalizerWithName(ctx context.Context, cli client.Client, obj client.Object, namespacedName types.NamespacedName, finalizerName string) error {
	err := cli.Get(ctx, namespacedName, obj)
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
		if err := cli.Update(ctx, obj); err != nil {
			return errors.Wrapf(err, "failed to remove finalizer %q on %q", finalizerName, accessor.GetName())
		}
	}

	return nil
}

// buildFinalizerName returns the finalizer name
func buildFinalizerName(kind string) string {
	return fmt.Sprintf("%s.%s", strings.ToLower(kind), curvev1.CustomResourceGroup)
}
