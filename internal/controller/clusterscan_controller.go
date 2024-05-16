/*
Copyright 2024.

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

package controller

import (
	"context"

	"github.com/go-logr/logr"
	batchv1 "k8s.io/api/batch/v1"
	batchv1beta1 "k8s.io/api/batch/v1beta1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	scanningv1alpha1 "github.com/jsulthan/clusterscan-controller/api/v1alpha1"
)

// ClusterScanReconciler reconciles a ClusterScan object
type ClusterScanReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=scanning.jam.clus.com,resources=clusterscans,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=scanning.jam.clus.com,resources=clusterscans/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=scanning.jam.clus.com,resources=clusterscans/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the ClusterScan object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.17.3/pkg/reconcile
func (r *ClusterScanReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("clusterscan", req.NamespacedName)

	var clusterScan scanningv1alpha1.ClusterScan
	if err := r.Get(ctx, req.NamespacedName, &clusterScan); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if clusterScan.Spec.Schedule != "" {
		// Create or update a CronJob for recurring scans
		cronJob := r.createCronJob(&clusterScan)
		if err := r.Create(ctx, cronJob); err != nil && !apierrors.IsAlreadyExists(err) {
			log.Error(err, "Failed to create CronJob")
			return ctrl.Result{}, err
		}
	} else {
		// Create a one-off Job for the scan
		job := r.createJob(&clusterScan)
		if err := r.Create(ctx, job); err != nil && !apierrors.IsAlreadyExists(err) {
			log.Error(err, "Failed to create Job")
			return ctrl.Result{}, err
		}
	}

	// Update the ClusterScan status
	clusterScan.Status.LastScanTime = metav1.Now()
	if err := r.Status().Update(ctx, &clusterScan); err != nil {
		log.Error(err, "Failed to update ClusterScan status")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *ClusterScanReconciler) createCronJob(clusterScan *scanningv1alpha1.ClusterScan) *batchv1beta1.CronJob {
	cronJob := &batchv1beta1.CronJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      clusterScan.Name,
			Namespace: clusterScan.Namespace,
		},
		Spec: batchv1beta1.CronJobSpec{
			Schedule: clusterScan.Spec.Schedule,
			JobTemplate: batchv1beta1.JobTemplateSpec{
				Spec: batchv1.JobSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name:  "scan",
									Image: clusterScan.Spec.ScanImage,
								},
							},
							RestartPolicy: corev1.RestartPolicyOnFailure,
						},
					},
				},
			},
		},
	}
	ctrl.SetControllerReference(clusterScan, cronJob, r.Scheme)
	return cronJob
}

func (r *ClusterScanReconciler) createJob(clusterScan *scanningv1alpha1.ClusterScan) *batchv1.Job {
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      clusterScan.Name,
			Namespace: clusterScan.Namespace,
		},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "scan",
							Image: clusterScan.Spec.ScanImage,
						},
					},
					RestartPolicy: corev1.RestartPolicyOnFailure,
				},
			},
		},
	}
	ctrl.SetControllerReference(clusterScan, job, r.Scheme)
	return job
}

// SetupWithManager sets up the controller with the Manager.
func (r *ClusterScanReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&scanningv1alpha1.ClusterScan{}).
		Complete(r)
}
