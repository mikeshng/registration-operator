package integration

import (
	"context"
	"fmt"

	"github.com/onsi/ginkgo"
	"github.com/onsi/gomega"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/openshift/library-go/pkg/controller/controllercmd"

	operatorapiv1 "github.com/open-cluster-management/api/operator/v1"
	"github.com/open-cluster-management/registration-operator/pkg/helpers"
	"github.com/open-cluster-management/registration-operator/pkg/operators"
	"github.com/open-cluster-management/registration-operator/test/integration/util"
)

func startHubOperator(ctx context.Context) {
	err := operators.RunClusterManagerOperator(ctx, &controllercmd.ControllerContext{
		KubeConfig:    restConfig,
		EventRecorder: util.NewIntegrationTestEventRecorder("integration"),
	})
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
}

var _ = ginkgo.Describe("ClusterManager", func() {
	var cancel context.CancelFunc
	var err error
	var clusterManagerName string
	var hubRegistrationClusterRole string
	var hubWebhookClusterRole string
	var hubRegistrationSA string
	var hubWebhookSA string
	var hubRegistrationDeployment string
	var hubWebhookDeployment string
	var webhookSecret string
	var validtingWebhook string

	ginkgo.BeforeEach(func() {
		var ctx context.Context
		ctx, cancel = context.WithCancel(context.Background())
		go startHubOperator(ctx)
	})

	ginkgo.AfterEach(func() {
		if cancel != nil {
			cancel()
		}
	})

	ginkgo.Context("Deploy and clean hub component", func() {
		ginkgo.BeforeEach(func() {
			clusterManagerName = "hub"
			hubRegistrationClusterRole = fmt.Sprintf("system:open-cluster-management:%s-registration-controller", clusterManagerName)
			hubWebhookClusterRole = fmt.Sprintf("system:open-cluster-management:%s-registration-webhook", clusterManagerName)
			hubRegistrationSA = fmt.Sprintf("%s-registration-controller-sa", clusterManagerName)
			hubWebhookSA = fmt.Sprintf("%s-registration-webhook-sa", clusterManagerName)
			hubRegistrationDeployment = fmt.Sprintf("%s-registration-controller", clusterManagerName)
			hubWebhookDeployment = fmt.Sprintf("%s-registration-webhook", clusterManagerName)
			webhookSecret = "webhook-serving-cert"
			validtingWebhook = "managedclustervalidators.admission.cluster.open-cluster-management.io"
		})

		ginkgo.It("should have expected resource created successfully", func() {
			clusterManager := &operatorapiv1.ClusterManager{
				ObjectMeta: metav1.ObjectMeta{
					Name: clusterManagerName,
				},
				Spec: operatorapiv1.ClusterManagerSpec{
					RegistrationImagePullSpec: "quay.io/open-cluster-management/registration",
				},
			}
			_, err = operatorClient.OperatorV1().ClusterManagers().Create(context.Background(), clusterManager, metav1.CreateOptions{})
			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			// Check namespace
			gomega.Eventually(func() bool {
				if _, err := kubeClient.CoreV1().Namespaces().Get(context.Background(), hubNamespace, metav1.GetOptions{}); err != nil {
					return false
				}
				return true
			}, eventuallyTimeout, eventuallyInterval).Should(gomega.BeTrue())

			// Check clusterrole/clusterrolebinding
			gomega.Eventually(func() bool {
				if _, err := kubeClient.RbacV1().ClusterRoles().Get(context.Background(), hubRegistrationClusterRole, metav1.GetOptions{}); err != nil {
					return false
				}
				return true
			}, eventuallyTimeout, eventuallyInterval).Should(gomega.BeTrue())
			gomega.Eventually(func() bool {
				if _, err := kubeClient.RbacV1().ClusterRoles().Get(context.Background(), hubWebhookClusterRole, metav1.GetOptions{}); err != nil {
					return false
				}
				return true
			}, eventuallyTimeout, eventuallyInterval).Should(gomega.BeTrue())
			gomega.Eventually(func() bool {
				if _, err := kubeClient.RbacV1().ClusterRoleBindings().Get(context.Background(), hubRegistrationClusterRole, metav1.GetOptions{}); err != nil {
					return false
				}
				return true
			}, eventuallyTimeout, eventuallyInterval).Should(gomega.BeTrue())
			gomega.Eventually(func() bool {
				if _, err := kubeClient.RbacV1().ClusterRoleBindings().Get(context.Background(), hubWebhookClusterRole, metav1.GetOptions{}); err != nil {
					return false
				}
				return true
			}, eventuallyTimeout, eventuallyInterval).Should(gomega.BeTrue())

			// Check service account
			gomega.Eventually(func() bool {
				if _, err := kubeClient.CoreV1().ServiceAccounts(hubNamespace).Get(context.Background(), hubRegistrationSA, metav1.GetOptions{}); err != nil {
					return false
				}
				return true
			}, eventuallyTimeout, eventuallyInterval).Should(gomega.BeTrue())
			gomega.Eventually(func() bool {
				if _, err := kubeClient.CoreV1().ServiceAccounts(hubNamespace).Get(context.Background(), hubWebhookSA, metav1.GetOptions{}); err != nil {
					return false
				}
				return true
			}, eventuallyTimeout, eventuallyInterval).Should(gomega.BeTrue())

			// Check deployment
			gomega.Eventually(func() bool {
				if _, err := kubeClient.AppsV1().Deployments(hubNamespace).Get(context.Background(), hubRegistrationDeployment, metav1.GetOptions{}); err != nil {
					return false
				}
				return true
			}, eventuallyTimeout, eventuallyInterval).Should(gomega.BeTrue())
			gomega.Eventually(func() bool {
				if _, err := kubeClient.AppsV1().Deployments(hubNamespace).Get(context.Background(), hubWebhookDeployment, metav1.GetOptions{}); err != nil {
					return false
				}
				return true
			}, eventuallyTimeout, eventuallyInterval).Should(gomega.BeTrue())

			// Check service
			gomega.Eventually(func() bool {
				if _, err := kubeClient.CoreV1().Services(hubNamespace).Get(context.Background(), hubWebhookDeployment, metav1.GetOptions{}); err != nil {
					return false
				}
				return true
			}, eventuallyTimeout, eventuallyInterval).Should(gomega.BeTrue())

			// Check webhook secret
			gomega.Eventually(func() bool {
				s, err := kubeClient.CoreV1().Secrets(hubNamespace).Get(context.Background(), webhookSecret, metav1.GetOptions{})
				if err != nil {
					return false
				}
				if s.Data == nil || s.Data["ca.crt"] == nil || s.Data["tls.crt"] == nil || s.Data["tls.key"] == nil {
					return false
				}
				return true
			}, eventuallyTimeout, eventuallyInterval).Should(gomega.BeTrue())

			// Check validating webhook
			gomega.Eventually(func() bool {
				if _, err := kubeClient.AdmissionregistrationV1().ValidatingWebhookConfigurations().Get(context.Background(), validtingWebhook, metav1.GetOptions{}); err != nil {
					return false
				}
				return true
			}, eventuallyTimeout, eventuallyInterval).Should(gomega.BeTrue())

			util.AssertClusterManagerCondition(clusterManagerName, operatorClient, "Applied", "ClusterManagerApplied", metav1.ConditionTrue)
		})
	})

	ginkgo.It("Deployment should be updated when clustermanager is changed", func() {
		gomega.Eventually(func() bool {
			if _, err := kubeClient.AppsV1().Deployments(hubNamespace).Get(context.Background(), hubRegistrationDeployment, metav1.GetOptions{}); err != nil {
				return false
			}
			return true
		}, eventuallyTimeout, eventuallyInterval).Should(gomega.BeTrue())

		// Check if generations are correct
		gomega.Eventually(func() bool {
			actual, err := operatorClient.OperatorV1().ClusterManagers().Get(context.Background(), clusterManagerName, metav1.GetOptions{})
			if err != nil {
				return false
			}

			if actual.Generation != actual.Status.ObservedGeneration {
				return false
			}

			return true
		}, eventuallyTimeout, eventuallyInterval).Should(gomega.BeTrue())

		clusterManager, err := operatorClient.OperatorV1().ClusterManagers().Get(context.Background(), clusterManagerName, metav1.GetOptions{})
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		clusterManager.Spec.RegistrationImagePullSpec = "testimage:latest"
		_, err = operatorClient.OperatorV1().ClusterManagers().Update(context.Background(), clusterManager, metav1.UpdateOptions{})
		gomega.Expect(err).NotTo(gomega.HaveOccurred())

		gomega.Eventually(func() bool {
			actual, err := kubeClient.AppsV1().Deployments(hubNamespace).Get(context.Background(), hubRegistrationDeployment, metav1.GetOptions{})
			if err != nil {
				return false
			}
			gomega.Expect(len(actual.Spec.Template.Spec.Containers)).Should(gomega.Equal(1))
			if actual.Spec.Template.Spec.Containers[0].Image != "testimage:latest" {
				return false
			}
			return true
		}, eventuallyTimeout, eventuallyInterval).Should(gomega.BeTrue())

		// Check if generations are correct
		gomega.Eventually(func() bool {
			actual, err := operatorClient.OperatorV1().ClusterManagers().Get(context.Background(), clusterManagerName, metav1.GetOptions{})
			if err != nil {
				return false
			}

			if actual.Generation != actual.Status.ObservedGeneration {
				return false
			}

			return true
		}, eventuallyTimeout, eventuallyInterval).Should(gomega.BeTrue())
	})

	ginkgo.It("Deployment should be reconciled when manually updated", func() {
		gomega.Eventually(func() bool {
			if _, err := kubeClient.AppsV1().Deployments(hubNamespace).Get(context.Background(), hubRegistrationDeployment, metav1.GetOptions{}); err != nil {
				return false
			}
			return true
		}, eventuallyTimeout, eventuallyInterval).Should(gomega.BeTrue())
		registrationoDeployment, err := kubeClient.AppsV1().Deployments(hubNamespace).Get(context.Background(), hubRegistrationDeployment, metav1.GetOptions{})
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		registrationoDeployment.Spec.Template.Spec.Containers[0].Image = "testimage2:latest"
		_, err = kubeClient.AppsV1().Deployments(hubNamespace).Update(context.Background(), registrationoDeployment, metav1.UpdateOptions{})
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		gomega.Eventually(func() bool {
			registrationoDeployment, err := kubeClient.AppsV1().Deployments(hubNamespace).Get(context.Background(), hubRegistrationDeployment, metav1.GetOptions{})
			if err != nil {
				return false
			}
			if registrationoDeployment.Spec.Template.Spec.Containers[0].Image != "testimage:latest" {
				return false
			}
			return true
		}, eventuallyTimeout, eventuallyInterval).Should(gomega.BeTrue())

		// Check if generations are correct
		gomega.Eventually(func() bool {
			actual, err := operatorClient.OperatorV1().ClusterManagers().Get(context.Background(), clusterManagerName, metav1.GetOptions{})
			if err != nil {
				return false
			}

			registrationoDeployment, err := kubeClient.AppsV1().Deployments(hubNamespace).Get(context.Background(), hubRegistrationDeployment, metav1.GetOptions{})
			if err != nil {
				return false
			}

			deploymentGeneration := helpers.NewGenerationStatus(appsv1.SchemeGroupVersion.WithResource("deployments"), registrationoDeployment)
			actualGeneration := helpers.FindGenerationStatus(actual.Status.Generations, deploymentGeneration)
			if deploymentGeneration.LastGeneration != actualGeneration.LastGeneration {
				return false
			}
			return true
		}, eventuallyTimeout, eventuallyInterval).Should(gomega.BeTrue())
	})
})
