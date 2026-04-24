/*
Copyright 2025.

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

package e2e

import (
	"fmt"
	"os/exec"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"everproc.com/devcontainer/test/utils"
)

// namespace where the project is deployed in
const namespace = "devcontainer-operator"
const featuresTestNamespace = "features-test"

var _ = Describe("Manager", Ordered, func() {
	var controllerPodName string
	testNSlist := []string{featuresTestNamespace}

	// Before running the tests, set up the environment by creating the namespace,
	// installing CRDs, and deploying the controller.
	BeforeAll(func() {
		By("cleaning up test namespaces")
		for _, ns := range testNSlist {
			cmd := exec.Command("kubectl", "delete", "ns", ns, "--ignore-not-found=true")
			_, _ = utils.Run(cmd)
			By("creating manager namespace")
			cmd = exec.Command("kubectl", "create", "ns", ns)
			_, err := utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Failed to create namespace")
		}
	})

	// After all tests have been executed, clean up by undeploying the controller, uninstalling CRDs,
	// and deleting the namespace.
	AfterAll(func() {
		for _, ns := range testNSlist {
			cmd := exec.Command("kubectl", "delete", "ns", ns, "--ignore-not-found=true")
			_, _ = utils.Run(cmd)
		}
	})

	// After each test, check for failures and collect logs, events,
	// and pod descriptions for debugging.
	AfterEach(func() {
		specReport := CurrentSpecReport()
		if specReport.Failed() {
			By("Fetching controller manager pod logs")
			if controllerPodName == "" {
				fmt.Println("Controller Pod Name is empty")
			}
			cmd := exec.Command("kubectl", "logs", controllerPodName, "-n", namespace)
			controllerLogs, err := utils.Run(cmd)
			if err == nil {
				_, _ = fmt.Fprintf(GinkgoWriter, "Controller logs:\n %s", controllerLogs)
			} else {
				_, _ = fmt.Fprintf(GinkgoWriter, "Failed to get Controller logs: %s", err)
			}

			By("Fetching Kubernetes events")
			cmd = exec.Command("kubectl", "get", "events", "-n", namespace, "--sort-by=.lastTimestamp")
			eventsOutput, err := utils.Run(cmd)
			if err == nil {
				_, _ = fmt.Fprintf(GinkgoWriter, "Kubernetes events:\n%s", eventsOutput)
			} else {
				_, _ = fmt.Fprintf(GinkgoWriter, "Failed to get Kubernetes events: %s", err)
			}

			By("Fetching controller manager pod description")
			cmd = exec.Command("kubectl", "describe", "pod", controllerPodName, "-n", namespace)
			podDescription, err := utils.Run(cmd)
			if err == nil {
				fmt.Println("Pod description:\n", podDescription)
			} else {
				fmt.Println("Failed to describe controller pod")
			}
		}
	})

	SetDefaultEventuallyTimeout(2 * time.Minute)
	SetDefaultEventuallyPollingInterval(time.Second)

	Context("Manager", func() {
		It("should run successfully", func() {
			By("validating that the controller-manager pod is running as expected")
			verifyControllerUp := func(g Gomega) {
				// Get the name of the controller-manager pod
				cmd := exec.Command("kubectl", "get",
					"pods", "-l", "control-plane=controller-manager",
					"-o", "go-template={{ range .items }}"+
						"{{ if not .metadata.deletionTimestamp }}"+
						"{{ .metadata.name }}"+
						"{{ \"\\n\" }}{{ end }}{{ end }}",
					"-n", namespace,
				)

				podOutput, err := utils.Run(cmd)
				fmt.Println(err)
				fmt.Println("output:", podOutput)
				g.Expect(err).NotTo(HaveOccurred(), "Failed to retrieve controller-manager pod information")
				podNames := utils.GetNonEmptyLines(podOutput)
				g.Expect(podNames).To(HaveLen(1), "expected 1 controller pod running")
				controllerPodName = podNames[0]
				g.Expect(controllerPodName).To(ContainSubstring("controller-manager"))
				g.Expect(controllerPodName).ToNot(BeEmpty())

				// Validate the pod's status
				cmd = exec.Command("kubectl", "get",
					"pods", controllerPodName, "-o", "jsonpath={.status.phase}",
					"-n", namespace,
				)
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal("Running"), "Incorrect controller-manager pod status")
			}
			Eventually(verifyControllerUp).Should(Succeed())
		})
	})

	Context("Basic Workspace Functionality", func() {
		It("should successfully create workspace and process devcontainer.json", func() {
			workspaceName := "workspace-basic-test"

			By("applying workspace with known working repository")
			workspaceYaml := fmt.Sprintf(`
apiVersion: devcontainer.everproc.com/v1alpha1
kind: Workspace
metadata:
  name: %s
  namespace: %s
spec:
  owner: test-user
  gitHashOrTag: master
  gitUrl: "https://github.com/daemonfire300/sample-jupyter-devcontainer.git"
  containerRegistry: "kind-registry:5000"
  insecureContainerRegistry: true`, workspaceName, featuresTestNamespace)

			cmd := exec.Command("kubectl", "apply", "-f", "-")
			cmd.Stdin = utils.StringToReader(workspaceYaml)
			_, err := utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Failed to create workspace")

			By("waiting for workspace to be ready")
			Eventually(func(g Gomega) {
				format := "jsonpath={.status.conditions[?(@.type=='Ready')].status}"
				cmd := exec.Command("kubectl", "get", "workspace", workspaceName, "-n", featuresTestNamespace, "-o", format)
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal("True"), "Workspace should be ready")
			}, 10*time.Minute, 30*time.Second).Should(Succeed())

			By("verifying definition was created")
			Eventually(func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "definitions", "-n", featuresTestNamespace)
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(ContainSubstring(workspaceName), "Definition should exist")
			}, 2*time.Minute, 10*time.Second).Should(Succeed())

			By("verifying deployment was created")
			Eventually(func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "deployment", "-n", featuresTestNamespace, "-l", "app.kubernetes.io/name=devcontainer")
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).NotTo(BeEmpty(), "Deployment should exist")
			}, 5*time.Minute, 10*time.Second).Should(Succeed())

			By("checking that the workspace completed successfully")
			format := "jsonpath={.status.conditions[?(@.type=='Ready')].message}"
			cmd = exec.Command("kubectl", "get", "workspace", workspaceName, "-n", featuresTestNamespace, "-o", format)
			output, err := utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())
			Expect(output).To(ContainSubstring("successfully"), "Workspace should complete successfully")

			By("cleaning up workspace")
			cmd = exec.Command("kubectl", "delete", "workspace", workspaceName, "-n", featuresTestNamespace)
			_, _ = utils.Run(cmd)
		})

	})

})
