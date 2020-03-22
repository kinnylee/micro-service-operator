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

package controllers

import (
	"context"
	appv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/api/extensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/go-logr/logr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	devopsv1 "kinnylee.com/micro-service-operator/api/v1"
)

// MicroServiceReconciler reconciles a MicroService object
type MicroServiceReconciler struct {
	client.Client
	Log logr.Logger
}

// +kubebuilder:rbac:groups=devops.kinnylee.com,resources=microservices,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=devops.kinnylee.com,resources=microservices/status,verbs=get;update;patch

func (r *MicroServiceReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	cxt := context.Background()
	log := r.Log.WithValues("microservice", req.NamespacedName)
	ms := &devopsv1.MicroService{}
	if err := r.Get(cxt, req.NamespacedName, ms); err != nil {
		if err := client.IgnoreNotFound(err); err != nil {
			log.Info("no resource microservice, 进入被删除成功后的生命周期")
			return ctrl.Result{}, nil
		} else {
			log.Error(err, "意料之外的错误")
			return ctrl.Result{}, err
		}
	}

	log.Info("MicroService被创建，根据crd的资源进入主流程")
	podLabels := map[string]string{
		"app": req.Name,
	}

	// create deployment
	deployment := appv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Deployment",
			APIVersion: "apps/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      req.Name,
			Namespace: req.Namespace,
		},
		Spec: appv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: podLabels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: podLabels,
				},
				Spec: corev1.PodSpec{
					ImagePullSecrets: []corev1.LocalObjectReference{
						{
							Name: ms.Spec.Secret,
						},
					},
					Containers: []corev1.Container{
						{
							Name:            req.Name,
							Image:           ms.Spec.Image,
							ImagePullPolicy: "Always",
							Ports: []corev1.ContainerPort{
								{
									ContainerPort: 8080,
								},
							},
						},
					},
				},
			},
		},
	}
	if err := r.Create(cxt, &deployment); err != nil {
		log.Error(err, "创建deployment资源出错")
		return ctrl.Result{}, nil
	}

	// create service
	service := corev1.Service{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Service",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      req.Name,
			Namespace: req.Namespace,
		},
		Spec: corev1.ServiceSpec{
			Selector: podLabels,
			Ports: []corev1.ServicePort{
				{
					Name:       req.Name,
					Port:       80,
					TargetPort: intstr.FromInt(8080),
					Protocol:   corev1.ProtocolTCP,
				},
			},
		},
		Status: corev1.ServiceStatus{},
	}
	if err := r.Create(cxt, &service); err != nil {
		log.Error(err, "创建service资源出错")
		return ctrl.Result{}, err
	}

	// create ingress
	ingress := v1beta1.Ingress{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Ingress",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      req.Name,
			Namespace: req.Namespace,
		},
		Spec: v1beta1.IngressSpec{
			Rules: []v1beta1.IngressRule{
				{
					Host: ms.Spec.Host,
					IngressRuleValue: v1beta1.IngressRuleValue{
						HTTP: &v1beta1.HTTPIngressRuleValue{
							Paths: []v1beta1.HTTPIngressPath{
								{
									Path: "/",
									Backend: v1beta1.IngressBackend{
										ServiceName: req.Name,
										ServicePort: intstr.FromInt(80),
									},
								},
							},
						},
					},
				},
			},
		},
		Status: v1beta1.IngressStatus{},
	}
	if err := r.Create(cxt, &ingress); err != nil {
		log.Error(err, "创建Ingress资源出错")
		return ctrl.Result{}, nil
	}
	return ctrl.Result{}, nil
}

func (r *MicroServiceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&devopsv1.MicroService{}).
		Complete(r)
}
