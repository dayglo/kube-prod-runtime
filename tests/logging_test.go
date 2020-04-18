/*
 * Bitnami Kubernetes Production Runtime - A collection of services that makes it
 * easy to run production workloads in Kubernetes.
 *
 * Copyright 2018-2019 Bitnami
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *   http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package integration

import (
	"encoding/json"
	"math/rand"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
)

var letterRunes = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")

func init() {
	rand.Seed(time.Now().UnixNano())
}

func RandString(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = letterRunes[rand.Intn(len(letterRunes))]
	}
	return string(b)
}

type total struct {
	Value int `json:"value"`
}

type hits struct {
	Total total `json:"total"`
}

type apiResponse struct {
	Hits hits `json:"hits"`
}

func totalHits(resp *apiResponse) int {
	return resp.Hits.Total.Value
}

// This test uses https://onsi.github.io/ginkgo/ - see there for docs
// on the slightly odd structure this imposes.
var _ = Describe("Logging", func() {
	var c kubernetes.Interface
	var deploy *appsv1.Deployment
	var ns string

	BeforeEach(func() {
		c = kubernetes.NewForConfigOrDie(clusterConfigOrDie())
		ns = createNsOrDie(c.CoreV1(), "test-logging-")

		decoder := scheme.Codecs.UniversalDeserializer()

		deploy = decodeFileOrDie(decoder, "testdata/logging-deploy.yaml").(*appsv1.Deployment)

		deploy.Spec.Template.Spec.Containers[0].Env[0].Value = RandString(32)
	})

	AfterEach(func() {
		deleteNs(c.CoreV1(), ns)
	})

	JustBeforeEach(func() {
		var err error
		deploy, err = c.AppsV1().Deployments(ns).Create(deploy)
		Expect(err).NotTo(HaveOccurred())
	})

	Context("basic", func() {
		// We create a container that repeatedly writes out a log signature to the
		// standard output and this test executes a query on elasticsearch to look up
		// the log signature
		It("should capture container logs", func() {
			Eventually(func() (*apiResponse, error) {
				var err error
				selector := "log:" + deploy.Spec.Template.Spec.Containers[0].Env[0].Value
				params := map[string]string{"q": selector}
				resultRaw, err := c.CoreV1().Services("kubeprod").ProxyGet("http", "elasticsearch-logging", "9200", "_search", params).DoRaw()
				if err != nil {
					return nil, err
				}

				resp := apiResponse{}
				json.Unmarshal(resultRaw, &resp)
				return &resp, err
			}, "10m", "5s").
				Should(WithTransform(totalHits, BeNumerically(">", 0)))
		})
	})
})
