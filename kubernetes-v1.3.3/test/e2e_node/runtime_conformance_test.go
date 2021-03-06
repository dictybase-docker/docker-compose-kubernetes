/*
Copyright 2016 The Kubernetes Authors All rights reserved.

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

package e2e_node

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"time"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/util"
	"k8s.io/kubernetes/test/e2e/framework"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

const (
	consistentCheckTimeout = time.Second * 5
	retryTimeout           = time.Minute * 5
	pollInterval           = time.Second * 1
)

var _ = framework.KubeDescribe("Container Runtime Conformance Test", func() {
	f := NewDefaultFramework("runtime-conformance")

	Describe("container runtime conformance blackbox test", func() {
		Context("when starting a container that exits", func() {
			It("it should run with the expected status [Conformance]", func() {
				restartCountVolumeName := "restart-count"
				restartCountVolumePath := "/restart-count"
				testContainer := api.Container{
					Image: ImageRegistry[busyBoxImage],
					VolumeMounts: []api.VolumeMount{
						{
							MountPath: restartCountVolumePath,
							Name:      restartCountVolumeName,
						},
					},
				}
				testVolumes := []api.Volume{
					{
						Name: restartCountVolumeName,
						VolumeSource: api.VolumeSource{
							HostPath: &api.HostPathVolumeSource{
								Path: os.TempDir(),
							},
						},
					},
				}
				testCases := []struct {
					Name          string
					RestartPolicy api.RestartPolicy
					Phase         api.PodPhase
					State         ContainerState
					RestartCount  int32
					Ready         bool
				}{
					{"terminate-cmd-rpa", api.RestartPolicyAlways, api.PodRunning, ContainerStateRunning, 2, true},
					{"terminate-cmd-rpof", api.RestartPolicyOnFailure, api.PodSucceeded, ContainerStateTerminated, 1, false},
					{"terminate-cmd-rpn", api.RestartPolicyNever, api.PodFailed, ContainerStateTerminated, 0, false},
				}
				for _, testCase := range testCases {
					tmpFile, err := ioutil.TempFile("", "restartCount")
					Expect(err).NotTo(HaveOccurred())
					defer os.Remove(tmpFile.Name())

					// It failed at the 1st run, then succeeded at 2nd run, then run forever
					cmdScripts := `
f=%s
count=$(echo 'hello' >> $f ; wc -l $f | awk {'print $1'})
if [ $count -eq 1 ]; then
	exit 1
fi
if [ $count -eq 2 ]; then
	exit 0
fi
while true; do sleep 1; done
`
					tmpCmd := fmt.Sprintf(cmdScripts, path.Join(restartCountVolumePath, path.Base(tmpFile.Name())))
					testContainer.Name = testCase.Name
					testContainer.Command = []string{"sh", "-c", tmpCmd}
					terminateContainer := ConformanceContainer{
						Container:     testContainer,
						Client:        f.Client,
						RestartPolicy: testCase.RestartPolicy,
						Volumes:       testVolumes,
						NodeName:      *nodeName,
						Namespace:     f.Namespace.Name,
					}
					Expect(terminateContainer.Create()).To(Succeed())
					defer terminateContainer.Delete()

					By("it should get the expected 'RestartCount'")
					Eventually(func() (int32, error) {
						status, err := terminateContainer.GetStatus()
						return status.RestartCount, err
					}, retryTimeout, pollInterval).Should(Equal(testCase.RestartCount))

					By("it should get the expected 'Phase'")
					Eventually(terminateContainer.GetPhase, retryTimeout, pollInterval).Should(Equal(testCase.Phase))

					By("it should get the expected 'Ready' condition")
					Expect(terminateContainer.IsReady()).Should(Equal(testCase.Ready))

					status, err := terminateContainer.GetStatus()
					Expect(err).ShouldNot(HaveOccurred())

					By("it should get the expected 'State'")
					Expect(GetContainerState(status.State)).To(Equal(testCase.State))

					By("it should be possible to delete [Conformance]")
					Expect(terminateContainer.Delete()).To(Succeed())
					Eventually(terminateContainer.Present, retryTimeout, pollInterval).Should(BeFalse())
				}
			})

			It("should report termination message if TerminationMessagePath is set [Conformance]", func() {
				name := "termination-message-container"
				terminationMessage := "DONE"
				terminationMessagePath := "/dev/termination-log"
				c := ConformanceContainer{
					Container: api.Container{
						Image:   ImageRegistry[busyBoxImage],
						Name:    name,
						Command: []string{"/bin/sh", "-c"},
						Args:    []string{fmt.Sprintf("/bin/echo -n %s > %s", terminationMessage, terminationMessagePath)},
						TerminationMessagePath: terminationMessagePath,
					},
					Client:        f.Client,
					RestartPolicy: api.RestartPolicyNever,
					NodeName:      *nodeName,
					Namespace:     f.Namespace.Name,
				}

				By("create the container")
				Expect(c.Create()).To(Succeed())
				defer c.Delete()

				By("wait for the container to succeed")
				Eventually(c.GetPhase, retryTimeout, pollInterval).Should(Equal(api.PodSucceeded))

				By("get the container status")
				status, err := c.GetStatus()
				Expect(err).NotTo(HaveOccurred())

				By("the container should be terminated")
				Expect(GetContainerState(status.State)).To(Equal(ContainerStateTerminated))

				By("the termination message should be set")
				Expect(status.State.Terminated.Message).Should(Equal(terminationMessage))

				By("delete the container")
				Expect(c.Delete()).To(Succeed())
			})
		})

		Context("when running a container with a new image", func() {
			// The service account only has pull permission
			auth := `
{
	"auths": {
		"https://gcr.io": {
			"auth": "X2pzb25fa2V5OnsKICAidHlwZSI6ICJzZXJ2aWNlX2FjY291bnQiLAogICJwcm9qZWN0X2lkIjogImF1dGhlbnRpY2F0ZWQtaW1hZ2UtcHVsbGluZyIsCiAgInByaXZhdGVfa2V5X2lkIjogImI5ZjJhNjY0YWE5YjIwNDg0Y2MxNTg2MDYzZmVmZGExOTIyNGFjM2IiLAogICJwcml2YXRlX2tleSI6ICItLS0tLUJFR0lOIFBSSVZBVEUgS0VZLS0tLS1cbk1JSUV2UUlCQURBTkJna3Foa2lHOXcwQkFRRUZBQVNDQktjd2dnU2pBZ0VBQW9JQkFRQzdTSG5LVEVFaVlMamZcbkpmQVBHbUozd3JCY2VJNTBKS0xxS21GWE5RL3REWGJRK2g5YVl4aldJTDhEeDBKZTc0bVovS01uV2dYRjVLWlNcbm9BNktuSU85Yi9SY1NlV2VpSXRSekkzL1lYVitPNkNjcmpKSXl4anFWam5mVzJpM3NhMzd0OUE5VEZkbGZycm5cbjR6UkpiOWl4eU1YNGJMdHFGR3ZCMDNOSWl0QTNzVlo1ODhrb1FBZmgzSmhhQmVnTWorWjRSYko0aGVpQlFUMDNcbnZVbzViRWFQZVQ5RE16bHdzZWFQV2dydDZOME9VRGNBRTl4bGNJek11MjUzUG4vSzgySFpydEx4akd2UkhNVXhcbng0ZjhwSnhmQ3h4QlN3Z1NORit3OWpkbXR2b0wwRmE3ZGducFJlODZWRDY2ejNZenJqNHlLRXRqc2hLZHl5VWRcbkl5cVhoN1JSQWdNQkFBRUNnZ0VBT3pzZHdaeENVVlFUeEFka2wvSTVTRFVidi9NazRwaWZxYjJEa2FnbmhFcG9cbjFJajJsNGlWMTByOS9uenJnY2p5VlBBd3pZWk1JeDFBZVF0RDdoUzRHWmFweXZKWUc3NkZpWFpQUm9DVlB6b3VcbmZyOGRDaWFwbDV0enJDOWx2QXNHd29DTTdJWVRjZmNWdDdjRTEyRDNRS3NGNlo3QjJ6ZmdLS251WVBmK0NFNlRcbmNNMHkwaCtYRS9kMERvSERoVy96YU1yWEhqOFRvd2V1eXRrYmJzNGYvOUZqOVBuU2dET1lQd2xhbFZUcitGUWFcbkpSd1ZqVmxYcEZBUW14M0Jyd25rWnQzQ2lXV2lGM2QrSGk5RXRVYnRWclcxYjZnK1JRT0licWFtcis4YlJuZFhcbjZWZ3FCQWtKWjhSVnlkeFVQMGQxMUdqdU9QRHhCbkhCbmM0UW9rSXJFUUtCZ1FEMUNlaWN1ZGhXdGc0K2dTeGJcbnplanh0VjFONDFtZHVjQnpvMmp5b1dHbzNQVDh3ckJPL3lRRTM0cU9WSi9pZCs4SThoWjRvSWh1K0pBMDBzNmdcblRuSXErdi9kL1RFalk4MW5rWmlDa21SUFdiWHhhWXR4UjIxS1BYckxOTlFKS2ttOHRkeVh5UHFsOE1veUdmQ1dcbjJ2aVBKS05iNkhabnY5Q3lqZEo5ZzJMRG5RS0JnUUREcVN2eURtaGViOTIzSW96NGxlZ01SK205Z2xYVWdTS2dcbkVzZlllbVJmbU5XQitDN3ZhSXlVUm1ZNU55TXhmQlZXc3dXRldLYXhjK0krYnFzZmx6elZZdFpwMThNR2pzTURcbmZlZWZBWDZCWk1zVXQ3Qmw3WjlWSjg1bnRFZHFBQ0xwWitaLzN0SVJWdWdDV1pRMWhrbmxHa0dUMDI0SkVFKytcbk55SDFnM2QzUlFLQmdRQ1J2MXdKWkkwbVBsRklva0tGTkh1YTBUcDNLb1JTU1hzTURTVk9NK2xIckcxWHJtRjZcbkMwNGNTKzQ0N0dMUkxHOFVUaEpKbTRxckh0Ti9aK2dZOTYvMm1xYjRIakpORDM3TVhKQnZFYTN5ZUxTOHEvK1JcbjJGOU1LamRRaU5LWnhQcG84VzhOSlREWTVOa1BaZGh4a2pzSHdVNGRTNjZwMVRESUU0MGd0TFpaRFFLQmdGaldcbktyblFpTnEzOS9iNm5QOFJNVGJDUUFKbmR3anhTUU5kQTVmcW1rQTlhRk9HbCtqamsxQ1BWa0tNSWxLSmdEYkpcbk9heDl2OUc2Ui9NSTFIR1hmV3QxWU56VnRocjRIdHNyQTB0U3BsbWhwZ05XRTZWejZuQURqdGZQSnMyZUdqdlhcbmpQUnArdjhjY21MK3dTZzhQTGprM3ZsN2VlNXJsWWxNQndNdUdjUHhBb0dBZWRueGJXMVJMbVZubEFpSEx1L0xcbmxtZkF3RFdtRWlJMFVnK1BMbm9Pdk81dFE1ZDRXMS94RU44bFA0cWtzcGtmZk1Rbk5oNFNZR0VlQlQzMlpxQ1RcbkpSZ2YwWGpveXZ2dXA5eFhqTWtYcnBZL3ljMXpmcVRaQzBNTzkvMVVjMWJSR2RaMmR5M2xSNU5XYXA3T1h5Zk9cblBQcE5Gb1BUWGd2M3FDcW5sTEhyR3pNPVxuLS0tLS1FTkQgUFJJVkFURSBLRVktLS0tLVxuIiwKICAiY2xpZW50X2VtYWlsIjogImltYWdlLXB1bGxpbmdAYXV0aGVudGljYXRlZC1pbWFnZS1wdWxsaW5nLmlhbS5nc2VydmljZWFjY291bnQuY29tIiwKICAiY2xpZW50X2lkIjogIjExMzc5NzkxNDUzMDA3MzI3ODcxMiIsCiAgImF1dGhfdXJpIjogImh0dHBzOi8vYWNjb3VudHMuZ29vZ2xlLmNvbS9vL29hdXRoMi9hdXRoIiwKICAidG9rZW5fdXJpIjogImh0dHBzOi8vYWNjb3VudHMuZ29vZ2xlLmNvbS9vL29hdXRoMi90b2tlbiIsCiAgImF1dGhfcHJvdmlkZXJfeDUwOV9jZXJ0X3VybCI6ICJodHRwczovL3d3dy5nb29nbGVhcGlzLmNvbS9vYXV0aDIvdjEvY2VydHMiLAogICJjbGllbnRfeDUwOV9jZXJ0X3VybCI6ICJodHRwczovL3d3dy5nb29nbGVhcGlzLmNvbS9yb2JvdC92MS9tZXRhZGF0YS94NTA5L2ltYWdlLXB1bGxpbmclNDBhdXRoZW50aWNhdGVkLWltYWdlLXB1bGxpbmcuaWFtLmdzZXJ2aWNlYWNjb3VudC5jb20iCn0=",
			"email": "image-pulling@authenticated-image-pulling.iam.gserviceaccount.com"
		}
	}
}`
			secret := &api.Secret{
				Data: map[string][]byte{api.DockerConfigJsonKey: []byte(auth)},
				Type: api.SecretTypeDockerConfigJson,
			}
			for _, testCase := range []struct {
				description string
				image       string
				secret      bool
				phase       api.PodPhase
				state       ContainerState
			}{
				{
					description: "should not be able to pull image from invalid registry",
					image:       "invalid.com/invalid/alpine:3.1",
					phase:       api.PodPending,
					state:       ContainerStateWaiting,
				},
				{
					description: "should not be able to pull non-existing image from gcr.io",
					image:       "gcr.io/google_containers/invalid-image:invalid-tag",
					phase:       api.PodPending,
					state:       ContainerStateWaiting,
				},
				{
					description: "should be able to pull image from gcr.io",
					image:       NoPullImageRegistry[pullTestAlpineWithBash],
					phase:       api.PodRunning,
					state:       ContainerStateRunning,
				},
				{
					description: "should be able to pull image from docker hub",
					image:       NoPullImageRegistry[pullTestAlpine],
					phase:       api.PodRunning,
					state:       ContainerStateRunning,
				},
				{
					description: "should not be able to pull from private registry without secret",
					image:       NoPullImageRegistry[pullTestAuthenticatedAlpine],
					phase:       api.PodPending,
					state:       ContainerStateWaiting,
				},
				{
					description: "should be able to pull from private registry with secret",
					image:       NoPullImageRegistry[pullTestAuthenticatedAlpine],
					secret:      true,
					phase:       api.PodRunning,
					state:       ContainerStateRunning,
				},
			} {
				testCase := testCase
				It(testCase.description, func() {
					name := "image-pull-test"
					command := []string{"/bin/sh", "-c", "while true; do sleep 1; done"}
					container := ConformanceContainer{
						Container: api.Container{
							Name:    name,
							Image:   testCase.image,
							Command: command,
							// PullAlways makes sure that the image will always be pulled even if it is present before the test.
							ImagePullPolicy: api.PullAlways,
						},
						Client:        f.Client,
						RestartPolicy: api.RestartPolicyNever,
						NodeName:      *nodeName,
						Namespace:     f.Namespace.Name,
					}
					if testCase.secret {
						secret.Name = "image-pull-secret-" + string(util.NewUUID())
						By("create image pull secret")
						_, err := f.Client.Secrets(f.Namespace.Name).Create(secret)
						Expect(err).NotTo(HaveOccurred())
						defer f.Client.Secrets(f.Namespace.Name).Delete(secret.Name)
						container.ImagePullSecrets = []string{secret.Name}
					}

					By("create the container")
					Expect(container.Create()).To(Succeed())
					defer container.Delete()

					By("check the pod phase")
					Eventually(container.GetPhase, retryTimeout, pollInterval).Should(Equal(testCase.phase))
					Consistently(container.GetPhase, consistentCheckTimeout, pollInterval).Should(Equal(testCase.phase))

					By("check the container state")
					status, err := container.GetStatus()
					Expect(err).NotTo(HaveOccurred())
					Expect(GetContainerState(status.State)).To(Equal(testCase.state))

					By("it should be possible to delete")
					Expect(container.Delete()).To(Succeed())
					Eventually(container.Present, retryTimeout, pollInterval).Should(BeFalse())
				})
			}
		})
	})
})
