/*

 Copyright 2021 The KubeVela Authors.

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

package workloads

import (
	"fmt"

	"github.com/crossplane/crossplane-runtime/pkg/event"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/oam-dev/kubevela/apis/core.oam.dev/v1beta1"
	"github.com/oam-dev/kubevela/apis/standard.oam.dev/v1alpha1"
	"github.com/oam-dev/kubevela/pkg/oam/util"
)

var _ = Describe("deployment controller", func() {
	var (
		c              DeploymentController
		ns             corev1.Namespace
		name           string
		namespace      string
		deploy         v1.Deployment
		namespacedName client.ObjectKey
	)
	Context("VerifySpec", func() {
		BeforeEach(func() {
			namespace = "rollout-ns"
			name = "rollout1"
			appRollout := v1beta1.AppRollout{ObjectMeta: metav1.ObjectMeta{Name: name}}
			namespacedName = client.ObjectKey{Name: name, Namespace: namespace}
			c = DeploymentController{
				client: k8sClient,
				rolloutSpec: &v1alpha1.RolloutPlan{
					RolloutBatches: []v1alpha1.RolloutBatch{{
						Replicas: intstr.FromInt(1),
					},
					},
				},
				rolloutStatus:    &v1alpha1.RolloutStatus{RollingState: v1alpha1.RolloutSucceedState},
				parentController: &appRollout,
				recorder: event.NewAPIRecorder(mgr.GetEventRecorderFor("AppRollout")).
					WithAnnotations("controller", "AppRollout"),
			}

			ns = corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: namespace,
				},
			}
			By("Create a namespace")
			Expect(k8sClient.Create(ctx, &ns)).Should(SatisfyAny(Succeed(), &util.AlreadyExistMatcher{}))
		})

		It("Could not fetch deployment workload", func() {
			consistent, err := c.VerifySpec(ctx)
			Expect(err).Should(BeNil())
			Expect(consistent).Should(BeFalse())
		})

		It("the source deployment is still being reconciled, need to be paused", func() {
			By("setting more fields for controller")
			c.sourceNamespacedName = namespacedName
			c.targetNamespacedName = namespacedName

			By("Create a deployment")
			deploy = v1.Deployment{
				TypeMeta:   metav1.TypeMeta{APIVersion: "apps/v1", Kind: "Deployment"},
				ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name},
				Spec: v1.DeploymentSpec{
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{"env": "staging"},
					},
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"env": "staging"}},
						Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: name, Image: "nginx"}}},
					},
				},
			}
			Expect(k8sClient.Create(ctx, &deploy)).Should(SatisfyAny(Succeed(), &util.AlreadyExistMatcher{}))

			By("verify")
			consistent, err := c.VerifySpec(ctx)
			Expect(err).Should(Equal(fmt.Errorf("the source deployment rollout1 is still being reconciled, need to be paused")))
			Expect(consistent).Should(BeFalse())

			By("clean up")
			var d v1.Deployment
			k8sClient.Get(ctx, namespacedName, &d)
			k8sClient.Delete(ctx, &d)
		})

		It("spec is valid", func() {
			By("setting more fields for controller")
			c.sourceNamespacedName = namespacedName
			c.targetNamespacedName = namespacedName

			By("Create a deployment")
			deploy = v1.Deployment{
				TypeMeta:   metav1.TypeMeta{APIVersion: "apps/v1", Kind: "Deployment"},
				ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name},
				Spec: v1.DeploymentSpec{
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{"env": "staging"},
					},
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"env": "staging"}},
						Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: name, Image: "nginx"}}},
					},
					Paused: true,
				},
			}
			Expect(k8sClient.Create(ctx, &deploy)).Should(Succeed())

			By("verify")
			consistent, err := c.VerifySpec(ctx)
			Expect(err).Should(BeNil())
			Expect(consistent).Should(BeTrue())

			By("clean up")
			var d v1.Deployment
			k8sClient.Get(ctx, namespacedName, &d)
			k8sClient.Delete(ctx, &d)
		})
	})
})
