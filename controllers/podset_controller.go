/*
Copyright 2021.

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

package controllers

import (
	"context"
	"reflect"
	"strconv"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"

	k8stestv1alpha1 "github.com/justin0u0/podset-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

// PodSetReconciler reconciles a PodSet object
type PodSetReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=k8stest.justin0u0.com,resources=podsets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=k8stest.justin0u0.com,resources=podsets/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=k8stest.justin0u0.com,resources=podsets/finalizers,verbs=update
// +kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;create;delete;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the PodSet object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.7.0/pkg/reconcile
func (r *PodSetReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("podset", req.NamespacedName)

	// Fetch podset instance
	podSet := &k8stestv1alpha1.PodSet{}
	if err := r.Get(ctx, req.NamespacedName, podSet); err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			log.Info("PodSet resource not found. Ingoring since object must be deleted.")
			return ctrl.Result{}, nil
		}
		// Error reading object - requeue the requst
		log.Error(err, "Failed to get PodSet resource.")
		return ctrl.Result{}, err
	}

	// Get all pods with label app=podSet.Name
	podList := &corev1.PodList{}
	labelSet := labels.Set{
		"app":       "podset",
		"podset_cr": podSet.Name, // podSet.ObjectMeta.Name
	}
	if err := r.List(ctx, podList, &client.ListOptions{
		Namespace:     req.Namespace, // req.NamespacedName.Namespace
		LabelSelector: labels.SelectorFromSet(labelSet),
	}); err != nil {
		log.Error(err, "Failed to list pods.")
		return ctrl.Result{}, err
	}

	// Get all running pods
	runningPods := []corev1.Pod{}
	runningPodNames := []string{}
	for _, pod := range podList.Items {
		if pod.ObjectMeta.DeletionTimestamp == nil && (pod.Status.Phase == corev1.PodPending || pod.Status.Phase == corev1.PodRunning) {
			runningPods = append(runningPods, pod)
			runningPodNames = append(runningPodNames, pod.Name)
		}
	}
	log.Info("Running pods", "runningPodNames", runningPodNames)
	log.Info("Excepting replicas", "PodSet.Spec.Replicas", podSet.Spec.Replicas)
	log.Info("Current replicas", "len(runningPodNames)", len(runningPodNames))

	// Update status if needed
	if newStatus := (k8stestv1alpha1.PodSetStatus{
		Replicas: int32(len(runningPodNames)),
		PodNames: runningPodNames,
	}); !reflect.DeepEqual(podSet.Status, newStatus) {
		podSet.Status = newStatus
		if err := r.Status().Update(ctx, podSet); err != nil {
			log.Error(err, "Failed to update PodSet status")
			return ctrl.Result{}, err
		}
	}

	// Scale down pods
	if int32(len(runningPodNames)) > podSet.Spec.Replicas {
		// Delete a pod once a time
		log.Info("Deleting a Pod in the PodSet", "PodSet.Name", podSet.Name)
		pod := runningPods[0]
		if err := r.Delete(ctx, &pod); err != nil {
			log.Error(err, "Failed to delete pod")
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	// Scale up pods
	if int32(len(runningPodNames)) < podSet.Spec.Replicas {
		// Create a pod once a time
		log.Info("Creating a Pod in the PodSet", "PodSet.Name", podSet.Name)
		pod := r.podForPodSet(podSet)

		if err := r.Create(ctx, pod); err != nil {
			log.Error(err, "Failed to create pod")
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	// All settled, no need requeue
	return ctrl.Result{}, nil
}

func (r *PodSetReconciler) podForPodSet(podSet *k8stestv1alpha1.PodSet) *corev1.Pod {
	labels := map[string]string{
		"app":       "podset",
		"podset_cr": podSet.Name,
	}

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podSet.Name + "-" + strconv.FormatInt(time.Now().UnixNano(), 36),
			Namespace: podSet.Namespace,
			Labels:    labels,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:    "busybox",
					Image:   "busybox",
					Command: []string{"sleep", "3600"},
				},
			},
		},
	}

	ctrl.SetControllerReference(podSet, pod, r.Scheme)
	return pod
}

// SetupWithManager sets up the controller with the Manager.
func (r *PodSetReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&k8stestv1alpha1.PodSet{}).
		Owns(&corev1.Pod{}).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: 2,
		}).
		Complete(r)
}
