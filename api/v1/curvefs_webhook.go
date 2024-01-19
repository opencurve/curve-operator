/*


Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1

import (
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

// log is for logging in this package.
var curvefslog = logf.Log.WithName("curvefs-resource")

func (r *Curvefs) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!

//+kubebuilder:webhook:path=/mutate-operator-curve-io-v1-curvefs,mutating=true,failurePolicy=fail,groups=operator.curve.io,resources=curvefs,verbs=create;update,versions=v1,name=mcurvefs.kb.io

var _ webhook.Defaulter = &Curvefs{}

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (r *Curvefs) Default() {
	curvefslog.Info("defaulting Curvefs", "Curvefs", client.ObjectKey{
		Name:      r.Name,
		Namespace: r.Namespace,
	})

	// TODO(user): fill in your defaulting logic.
}

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
//+kubebuilder:webhook:verbs=create;update;delete,path=/validate-operator-curve-io-v1-curvefs,mutating=false,failurePolicy=fail,groups=operator.curve.io,resources=curvefs,versions=v1,name=vcurvefs.kb.io

var _ webhook.Validator = &Curvefs{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *Curvefs) ValidateCreate() error {
	curvefslog.Info("validating creation of Curvefs", "Curvefs", client.ObjectKey{
		Name:      r.Name,
		Namespace: r.Namespace,
	})

	// TODO(user): fill in your validation logic upon object creation.
	return nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *Curvefs) ValidateUpdate(old runtime.Object) error {
	curvefslog.Info("validating update of Curvefs", "Curvefs", client.ObjectKey{
		Name:      r.Name,
		Namespace: r.Namespace,
	})

	// TODO(user): fill in your validation logic upon object update.
	return nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *Curvefs) ValidateDelete() error {
	curvefslog.Info("validating deletion of Curvefs", "Curvefs", client.ObjectKey{
		Name:      r.Name,
		Namespace: r.Namespace,
	})

	// TODO(user): fill in your validation logic upon object deletion.
	return nil
}
