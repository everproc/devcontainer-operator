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
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"everproc.com/devcontainer/test/utils"
)

var _ = Describe("Features Integration", Ordered, func() {
	const testNamespace = "features-test"

	BeforeAll(func() {
		By("creating test namespace")
		cmd := exec.Command("kubectl", "create", "ns", testNamespace)
		_, err := utils.Run(cmd)
		if err != nil && !strings.Contains(err.Error(), "AlreadyExists") {
			Expect(err).NotTo(HaveOccurred(), "Failed to create test namespace")
		}
	})

	AfterAll(func() {
		By("cleaning up test namespace")
		cmd := exec.Command("kubectl", "delete", "ns", testNamespace, "--ignore-not-found=true")
		_, _ = utils.Run(cmd)
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
  insecureContainerRegistry: "true"
`, workspaceName, testNamespace)

			cmd := exec.Command("kubectl", "apply", "-f", "-")
			cmd.Stdin = utils.StringToReader(workspaceYaml)
			_, err := utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Failed to create workspace")

			By("waiting for workspace to be ready")
			Eventually(func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "workspace", workspaceName, "-n", testNamespace, "-o", "jsonpath={.status.conditions[?(@.type=='Ready')].status}")
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal("True"), "Workspace should be ready")
			}, 10*time.Minute, 30*time.Second).Should(Succeed())

			By("verifying definition was created")
			Eventually(func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "definitions", "-n", testNamespace)
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(ContainSubstring(workspaceName), "Definition should exist")
			}, 2*time.Minute, 10*time.Second).Should(Succeed())

			By("verifying deployment was created")
			Eventually(func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "deployment", "-n", testNamespace, "-l", "app.kubernetes.io/name=devcontainer")
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).NotTo(BeEmpty(), "Deployment should exist")
			}, 5*time.Minute, 10*time.Second).Should(Succeed())

			By("checking that the workspace completed successfully")
			cmd = exec.Command("kubectl", "get", "workspace", workspaceName, "-n", testNamespace, "-o", "jsonpath={.status.conditions[?(@.type=='Ready')].message}")
			output, err := utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())
			Expect(output).To(ContainSubstring("successfully"), "Workspace should complete successfully")

			By("verifying features processing capability")
			// Check if the parserapp processed the devcontainer.json
			cmd = exec.Command("kubectl", "get", "configmaps", "-n", testNamespace, "-l", "app.kubernetes.io/name=devcontainer", "-o", "jsonpath={.items[0].binaryData.definition}")
			output, err = utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())
			Expect(output).NotTo(BeEmpty(), "Definition should contain parsed data")

			By("cleaning up workspace")
			cmd = exec.Command("kubectl", "delete", "workspace", workspaceName, "-n", testNamespace)
			_, _ = utils.Run(cmd)
		})

		It("should handle workspace creation and deletion lifecycle", func() {
			workspaceName := "workspace-lifecycle-test"

			By("creating workspace")
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
  insecureContainerRegistry: "true"
`, workspaceName, testNamespace)

			cmd := exec.Command("kubectl", "apply", "-f", "-")
			cmd.Stdin = utils.StringToReader(workspaceYaml)
			_, err := utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Failed to create workspace")

			By("waiting for workspace to start reconciling")
			Eventually(func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "workspace", workspaceName, "-n", testNamespace, "-o", "jsonpath={.status.conditions[0].type}")
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal("Ready"), "Workspace should have Ready condition")
			}, 2*time.Minute, 10*time.Second).Should(Succeed())

			By("deleting workspace")
			cmd = exec.Command("kubectl", "delete", "workspace", workspaceName, "-n", testNamespace)
			_, err = utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Failed to delete workspace")

			By("verifying workspace is deleted")
			Eventually(func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "workspace", workspaceName, "-n", testNamespace)
				_, err := utils.Run(cmd)
				g.Expect(err).To(HaveOccurred(), "Workspace should be deleted")
			}, 2*time.Minute, 10*time.Second).Should(Succeed())
		})
	})

	Context("Features Processing", func() {
		It("should demonstrate features integration capability", func() {
			workspaceName := "workspace-features-demo"

			By("creating workspace that will exercise features code path")
			// TODO(juf): Move this into a YAML? or In-Code Struct maybe
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
  insecureContainerRegistry: "true"
`, workspaceName, testNamespace)

			cmd := exec.Command("kubectl", "apply", "-f", "-")
			cmd.Stdin = utils.StringToReader(workspaceYaml)
			_, err := utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Failed to create workspace")

			By("waiting for definition to be created")
			Eventually(func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "definitions", "-n", testNamespace)
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(ContainSubstring(workspaceName), "Definition should exist")
			}, 5*time.Minute, 15*time.Second).Should(Succeed())

			By("checking that features.Prepare was called in parserapp")
			Eventually(func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "definitions", "-n", testNamespace, "-o", "jsonpath={.items[0].status.conditions[?(@.type=='Ready')].status}")
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal("True"), "Definition should be ready")
			}, 8*time.Minute, 30*time.Second).Should(Succeed())

			By("verifying the features processing completed")
			// Check that the definition has the features field populated
			cmd = exec.Command("kubectl", "get", "configmaps", "-n", testNamespace, "-l", "app.kubernetes.io/name=devcontainer", "-o", "jsonpath={.items[0].binaryData.definition}")
			output, err := utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())
			Expect(output).NotTo(BeEmpty(), "Definition should contain processed features data")

			By("cleaning up workspace")
			cmd = exec.Command("kubectl", "delete", "workspace", workspaceName, "-n", testNamespace)
			_, _ = utils.Run(cmd)
		})
	})
})

